// Copyright 2017 Applatix, Inc.
package costdb

import (
	"encoding/json"
	"fmt"
	"log"
	"sort"
	"strings"
	"time"

	"github.com/applatix/claudia"
	"github.com/applatix/claudia/errors"
	"github.com/applatix/claudia/parser"
	client "github.com/influxdata/influxdb/client/v2"
	"github.com/influxdata/influxdb/models"
	"github.com/influxdata/influxdb/toml"
)

// CostDatabase provides an interface to store and query cost and usage information from a cost database
type CostDatabase struct {
	ServerURL    string
	client       client.Client
	databaseName string
}

// CostReportContext provides a querying interface in the context of a user report
type CostReportContext struct {
	CostDB              *CostDatabase
	ReportID            string
	measurementName     string
	fqMeasurementName   string
	retentionPolicyName string
}

// Interval when aggregating data
const (
	Hour  Interval = "1h"
	Day   Interval = "1d"
	Week  Interval = "1w"
	Month Interval = "1M"
)

// Interval is a time interval used for grouping cost queries
type Interval string

// ParseInterval parses a string as an interval
func ParseInterval(intervalString string) (Interval, error) {
	switch intervalString {
	case "1h":
		return Hour, nil
	case "1d":
		return Day, nil
	case "1w":
		return Week, nil
	case "1M":
		return Month, nil
	default:
		return "", fmt.Errorf("Invalid interval: %s", intervalString)
	}
}

// CostQuery is the object representation of a cost database query for cost or usages
// * Aggregator is a format string, e.g: "COUNT(DISTINCT\"%s\"))"
// * Field is the column to query against, e.g. "lineItem/UnblendedCost""
// * From/To is the timeframe in which to perform the query
// * GroupBy is the column name in which to group the query by
// * Filters will is a mapping of column names to values in which to filter by
type CostQuery struct {
	Aggregator string
	Field      string
	From       time.Time
	To         time.Time
	GroupBy    string
	Interval   Interval
	Blended    bool
	Filters    map[string][]string
}

// NewCostDatabase returns a CostDatabase instance
func NewCostDatabase(dbURL string) (costDb *CostDatabase, err error) {
	influxClient, err := client.NewHTTPClient(client.HTTPConfig{
		Addr: dbURL,
	})
	if err != nil {
		return nil, errors.InternalError(err)
	}
	costDb = &CostDatabase{dbURL, influxClient, claudia.CostDatabaseName}
	return costDb, nil
}

// Wait will wait until database is ready
func (db *CostDatabase) Wait() {
	log.Printf("Waiting for cost database to be ready")
	for {
		_, _, err := db.client.Ping(20 * time.Second)
		if err == nil {
			break
		}
		time.Sleep(time.Second)
	}
	log.Printf("Cost database is ready")
}

// CreateDatabase create the database
func (db *CostDatabase) CreateDatabase() error {
	log.Printf("Creating database %s", db.databaseName)
	_, err := db.Query("CREATE DATABASE %s", db.databaseName)
	return err
}

// DropDatabase will drop entire database
func (db *CostDatabase) DropDatabase() error {
	log.Printf("Dropping database %s", db.databaseName)
	_, err := db.Query("DROP DATABASE %s", db.databaseName)
	return err
}

// Query performs a InfluxDB query
func (db *CostDatabase) Query(cmd string, args ...interface{}) ([]client.Result, error) {
	cmd = fmt.Sprintf(cmd, args...)
	q := client.Query{
		Command:  cmd,
		Database: db.databaseName,
	}
	log.Println(cmd)
	response, err := db.client.Query(q)
	if err != nil {
		return nil, errors.InternalErrorf(err, "Query '%s' failed", cmd)
	}
	err = errors.InternalErrorf(response.Error(), "Query '%s' had error response", cmd)
	if err != nil {
		return nil, err
	}
	return response.Results, nil
}

// Write takes a BatchPoints object and writes all Points to InfluxDB while retrying the operation several times
func (db *CostDatabase) Write(bp client.BatchPoints) error {
	var err error
	for _, delay := range claudia.IngestdWriteRetryDelay {
		err = db.client.Write(bp)
		if err == nil {
			return nil
		}
		log.Printf("Write failed due to: %s. Retrying in %fs", err, delay.Seconds())
		time.Sleep(delay)
	}
	return errors.InternalError(db.client.Write(bp))
}

// NewCostReportContext returns a context in which to perform cost queries
func (db *CostDatabase) NewCostReportContext(reportID string) *CostReportContext {
	measurementName := fmt.Sprintf("report_%s", reportID)
	retentionPolicyName := fmt.Sprintf("rtn_%s", reportID)
	fqMeasurementName := fmt.Sprintf(`%s."%s"."%s"`, db.databaseName, retentionPolicyName, measurementName)
	return &CostReportContext{
		CostDB:              db,
		ReportID:            reportID,
		measurementName:     measurementName,
		fqMeasurementName:   fqMeasurementName,
		retentionPolicyName: retentionPolicyName,
	}
}

// NewBatchPoints returns a new batch points with the database and retention policy set.
// NOTE: precision is left as nanoseconds despite billing reports only having hourly report granularity, because we use
// a nanosecond sequence number added to each timestamp to ensure we do not lose any points from InfluxDB's point
// duplication logic. Per recomendation, we can increase the timestamp by a nanosecond to prevent point duplication. See:
// https://docs.influxdata.com/influxdb/v1.1/troubleshooting/frequently-asked-questions/#how-does-influxdb-handle-duplicate-points
func (ctx *CostReportContext) NewBatchPoints() (client.BatchPoints, error) {
	bp, err := client.NewBatchPoints(client.BatchPointsConfig{
		Database:        ctx.CostDB.databaseName,
		RetentionPolicy: ctx.retentionPolicyName,
		Precision:       "ns",
	})
	if err != nil {
		return nil, errors.InternalError(err)
	}
	return bp, nil
}

// TagValues return tag values of a particular column. Filters are mapping from column name to value
func (ctx *CostReportContext) TagValues(column parser.Column, filters map[string][]string) ([]string, error) {
	query := fmt.Sprintf("SHOW TAG VALUES FROM %s WITH KEY=\"%s\"", ctx.fqMeasurementName, column.ColumnName)
	filterQuery, err := constructFilterQuery(filters)
	if err != nil {
		return nil, err
	}
	if len(filterQuery) > 0 {
		query += " WHERE " + strings.Join(filterQuery, " AND ")
	}
	res, err := ctx.CostDB.Query(query)
	if err != nil {
		return nil, err
	}
	if len(res[0].Series) == 0 {
		return make([]string, 0), nil
	}
	tagValues := make([]string, len(res[0].Series[0].Values))
	for i, tuple := range res[0].Series[0].Values {
		tagVal := tuple[1].(string)
		tagValues[i] = tagVal
	}
	return tagValues, nil
}

func constructFilterQuery(filters map[string][]string) ([]string, error) {
	queries := make([]string, 0)
	for columnName, filterValues := range filters {
		column := parser.GetColumnByName(columnName)
		if column == nil && !strings.HasPrefix(columnName, "resourceTags") {
			return nil, errors.Errorf(errors.CodeBadRequest, "Column %s does not exist", columnName)
		}
		colFilters := make([]string, 0)
		for _, colVal := range filterValues {
			// tag should be double quoted, value single quoted
			colFilters = append(colFilters, fmt.Sprintf("\"%s\"='%s'", columnName, colVal))
		}
		queries = append(queries, fmt.Sprintf("(%s)", strings.Join(colFilters, " OR ")))
	}
	return queries, nil
}

// Cost perform a cost query
func (ctx *CostReportContext) Cost(params *CostQuery) ([]models.Row, error) {
	var field string
	if params.Field == "" {
		field = parser.ColumnUnblendedCost.ColumnName
	} else {
		field = params.Field
	}
	var selector string
	if params.Aggregator == "" {
		selector = fmt.Sprintf("SUM(\"%s\")", field)
	} else {
		selector = fmt.Sprintf(params.Aggregator, field)
	}
	query := fmt.Sprintf("SELECT %s FROM %s", selector, ctx.fqMeasurementName)

	filters := make([]string, 0)
	if !params.From.IsZero() {
		filters = append(filters, fmt.Sprintf("time >= '%s'", params.From.Format(time.RFC3339)))
	}
	if !params.To.IsZero() {
		// NOTE: the API only accepts date ranges (i.e. not at the time/hour level). The date ranges are
		// inclusive to the entire end date (e.g. 2017-01-01 to 2017-01-31 should include data from 1/31).
		// To achieve this, add one day to the 'To' param, truncate any time information, and use the '<' operator to InfluxDB.
		toDate := time.Date(params.To.Year(), params.To.Month(), params.To.Day(), 0, 0, 0, 0, params.To.Location()).Add(24 * time.Hour)
		filters = append(filters, fmt.Sprintf("time < '%s'", toDate.Format(time.RFC3339)))
	}
	filterQuery, err := constructFilterQuery(params.Filters)
	if err != nil {
		return nil, err
	}
	if len(filterQuery) > 0 {
		filters = append(filters, filterQuery...)
	} else {
		// This will remove any zero value rows from query
		switch field {
		case parser.ColumnUnblendedCost.ColumnName, parser.ColumnBlendedCost.ColumnName, parser.ColumnUsageAmount.ColumnName:
			filters = append(filters, fmt.Sprintf("\"%s\" > 0", field))
		}
	}
	if len(filters) > 0 {
		query += " WHERE " + strings.Join(filters, " AND ")
	}

	groupings := make([]string, 0)
	// Handle groupings (e.g. account, product, etc...)
	if params.GroupBy != "" {
		columnName := parser.APINameToColumnName(params.GroupBy)
		if columnName == nil {
			return nil, errors.Errorf(errors.CodeBadRequest, "Invalid group by: %s", params.GroupBy)
		}
		groupings = append(groupings, "\""+*columnName+"\"")
	}
	// Handle interval
	monthlyRollup := false
	if params.Interval != "" {
		if params.Interval == Month {
			// For monthly interval, need to perform roll up ourselves since InfluxDB does not support it
			groupings = append(groupings, fmt.Sprintf("time(%s)", Day))
			monthlyRollup = true
		} else if params.Interval == Week {
			// Weekly groupings need offset for dates to start on Sunday
			// See: https://github.com/influxdata/influxdb/pull/387
			groupings = append(groupings, fmt.Sprintf("time(%s, 3d)", Week))
		} else {
			groupings = append(groupings, fmt.Sprintf("time(%s)", params.Interval))
		}
	}
	if len(groupings) > 0 {
		query += " GROUP BY " + strings.Join(groupings, ",")
	}
	query += " fill(0)"
	log.Println("Query: ", query)
	res, err := ctx.CostDB.Query(query)
	if err != nil {
		return nil, err
	}
	rows := res[0].Series
	if len(rows) > 0 && rows[len(rows)-1].Partial {
		// The partial flag indicates if InfluxDB truncated the result due to reaching max-row-limit (tuned to: 20000)
		return nil, errors.New(errors.CodeForbidden, "Query returned too many data points. Apply additional filters, increase interval, or reduce time range")
	}
	if monthlyRollup {
		err := rollUpMonthly(rows)
		if err != nil {
			return nil, err
		}
	}
	return rows, nil
}

// To support sorting by timestamps
type timeSlice []time.Time

func (s timeSlice) Less(i, j int) bool { return s[i].Before(s[j]) }
func (s timeSlice) Swap(i, j int)      { s[i], s[j] = s[j], s[i] }
func (s timeSlice) Len() int           { return len(s) }

// Helper to mutate the rows by rolling up Values of time series into logical monthly time boundaries
// models.Row has the following example datastructure
//[
//  {
//    "name": "cost",
//    "tags": {"lineItem/UsageAccountId": "012345678910"},
//    "columns": ["time", "sum"],
//    "values": [
//      ["2016-10-31T23:00:00Z", 1061.9235875300074],
//      ["2016-11-01T00:00:00Z", 2465.9980534299034],
//      ["2016-11-01T01:00:00Z", 1743.5425535196027],
//      ["2016-11-01T00:00:00Z", 0]
//    ]
//  },
//	{
//    "name": "cost",
//    "tags": {"lineItem/UsageAccountId": "246810121416"},
//    "columns": ["time", "sum"],
//    "values": [
//      ["2016-10-30T23:00:00Z", 316.7869662999978],
//      ["2016-11-01T00:00:00Z", 679.0124672099886],
//      ["2016-11-01T01:00:00Z", 355.3621549299981],
//      ["2016-11-01T02:00:00Z", 0]
//    ]
//  }
//]
func rollUpMonthly(rows []models.Row) error {
	log.Println("Rolling up monthly")
	for i, row := range rows {
		var monthlyTotals = make(map[time.Time]float64)
		for _, valueTuple := range row.Values {
			timestamp, err := time.Parse(time.RFC3339, valueTuple[0].(string))
			if err != nil {
				return errors.InternalError(err)
			}
			tsTruncated := time.Date(timestamp.Year(), timestamp.Month(), 1, 0, 0, 0, 0, timestamp.Location())
			value, err := valueTuple[1].(json.Number).Float64()
			if err != nil {
				return errors.InternalError(err)
			}
			prevValue, exists := monthlyTotals[tsTruncated]
			if !exists {
				monthlyTotals[tsTruncated] = value
			} else {
				monthlyTotals[tsTruncated] = prevValue + value
			}
		}
		// Get months in sorted order
		var keys timeSlice
		for k := range monthlyTotals {
			keys = append(keys, k)
		}
		sort.Sort(keys)

		rows[i].Values = make([][]interface{}, len(monthlyTotals))
		for j, monthTs := range keys {
			monthTotal, _ := monthlyTotals[monthTs]
			tuple := []interface{}{monthTs, monthTotal}
			rows[i].Values[j] = tuple
		}
	}
	return nil
}

// CountRecords counts the total number of records in the measurement
func (ctx *CostReportContext) CountRecords() (int64, error) {
	res, err := ctx.CostDB.Query("SELECT count(\"%s\") FROM %s", parser.ColumnUnblendedCost.ColumnName, ctx.fqMeasurementName)
	if err != nil {
		return -1, err
	}
	if len(res[0].Series) <= 0 {
		return 0, nil
	}
	count, _ := res[0].Series[0].Values[0][1].(json.Number).Int64()
	return count, nil
}

// SeriesCardinality Return cardinality of the measurement
func (ctx *CostReportContext) SeriesCardinality() (int, error) {
	//res, err := ctx.CostDB.Query(`SELECT numSeries FROM "_internal".."database" WHERE time > now() - 10s GROUP BY "database" ORDER BY desc LIMIT 1`)
	res, err := ctx.CostDB.Query("SHOW SERIES FROM \"%s\"", ctx.measurementName)
	if err != nil {
		return -1, err
	}
	if len(res) == 0 || len(res[0].Series) == 0 {
		return 0, nil
	}
	return len(res[0].Series[0].Values), nil
}

// DeleteReportData deletes all report data for the measurement
func (ctx *CostReportContext) DeleteReportData() error {
	log.Printf("Deleting report data for %s", ctx.measurementName)
	err := ctx.DeleteAllIngestHistory()
	if err != nil {
		// Influx sdk does not provide a constant for db not found
		origErr := errors.Cause(err)
		if strings.Contains(strings.ToLower(origErr.Error()), "database not found") {
			return nil
		}
		return err
	}
	// From docs: If you attempt to drop a retention policy that does not exist, InfluxDB does not return an error.
	err = ctx.DropRetentionPolicy()
	if err != nil {
		return err
	}
	_, err = ctx.CostDB.Query("DROP MEASUREMENT \"%s\"", ctx.measurementName)
	if err != nil {
		// Influx sdk does not provide a constant for measurement not found
		origErr := errors.Cause(err)
		if strings.Contains(strings.ToLower(origErr.Error()), "measurement not found") {
			return nil
		}
	}
	return err
}

// DeleteReportBillingBucketData deletes all report data for a specific billing bucket/report prefix
func (ctx *CostReportContext) DeleteReportBillingBucketData(bucketname, reportPrefix string) error {
	log.Printf("Deleting report data for %s (bucket: %s, reportPrefix: %s)", ctx.measurementName, bucketname, reportPrefix)
	err := ctx.DeleteBillingBucketHistory(bucketname, reportPrefix)
	if err != nil {
		// Influx sdk does not provide a constant for db not found
		origErr := errors.Cause(err)
		if strings.Contains(strings.ToLower(origErr.Error()), "database not found") {
			return nil
		}
		return err
	}
	_, err = ctx.CostDB.Query("DROP SERIES FROM \"%s\" WHERE \"%s\"='%s' AND \"%s\"='%s'",
		ctx.measurementName,
		parser.ColumnBillingBucket.ColumnName, bucketname,
		parser.ColumnBillingReportPathPrefix.ColumnName, escapeSingleQuote(reportPrefix))
	return err
}

// PurgeBillingPeriodSeries will drop data points corresponding to the given billing period
// This is desired for when current month's data needs to be replaced (new report is generated)
func (ctx *CostReportContext) PurgeBillingPeriodSeries(bucketname, reportPrefix, billingPeriod string) error {
	_, err := ctx.CostDB.Query("DROP SERIES FROM \"%s\" WHERE \"%s\"='%s' AND \"%s\"='%s' AND \"%s\"='%s'",
		ctx.measurementName,
		parser.ColumnBillingBucket.ColumnName, bucketname,
		parser.ColumnBillingReportPathPrefix.ColumnName, escapeSingleQuote(reportPrefix),
		parser.ColumnBillingPeriod.ColumnName, billingPeriod)
	return err
}

// CreateRetentionPolicy creates the retention policy for this report
func (ctx *CostReportContext) CreateRetentionPolicy(days int) error {
	_, err := ctx.CostDB.Query("CREATE RETENTION POLICY \"%s\" ON \"%s\" DURATION %dd REPLICATION 1",
		ctx.retentionPolicyName, ctx.CostDB.databaseName, days)
	return err
}

// GetRetentionPolicy gets the retention policy for this report in days
func (ctx *CostReportContext) GetRetentionPolicy() (int, error) {
	res, err := ctx.CostDB.Query("SHOW RETENTION POLICIES ON \"%s\"", ctx.CostDB.databaseName)
	if err != nil {
		return -1, err
	}
	if len(res) == 0 || len(res[0].Series) == 0 {
		return -1, errors.Errorf("No retention policy found for report %s", ctx.ReportID)
	}
	row := res[0].Series[0]
	durationIndex := -1
	nameIndex := -1
	for i, columnName := range row.Columns {
		if columnName == "duration" {
			durationIndex = i
		}
		if columnName == "name" {
			nameIndex = i
		}
	}
	for _, val := range row.Values {
		retentionPolicyName := val[nameIndex]
		if retentionPolicyName == ctx.retentionPolicyName {
			durationString := val[durationIndex].(string) // (e.g. 8760h0m0s)
			var tomlDuration toml.Duration
			err = tomlDuration.UnmarshalText([]byte(durationString))
			if err != nil {
				return -1, errors.InternalError(err)
			}
			durationDays := int(time.Duration(tomlDuration).Hours() / 24)
			return durationDays, nil
		}
	}
	return -1, errors.Errorf("No retention policy found for report %s", ctx.ReportID)
}

// UpdateRetentionPolicy updates the retention policy for this report.
// If retention is increased, it purges ingest_status history during the the increased duration time period
func (ctx *CostReportContext) UpdateRetentionPolicy(days int) (bool, error) {
	existingRetention, err := ctx.GetRetentionPolicy()
	if err != nil {
		return false, err
	}
	if existingRetention != days {
		if days > existingRetention {
			prevCutoff := time.Now().AddDate(0, 0, -existingRetention)
			//newCutoff := time.Now().AddDate(0, 0, -days)
			ingestStatuses, err := ctx.GetReportIngestStatuses()
			if err != nil {
				return false, err
			}
			// Delete ingest status for any buckets occurring during the increased time period.
			// This will force (re)processing of the buckets
			for _, ingStatus := range ingestStatuses {
				parts := strings.Split(ingStatus.BillingPeriod, "-")
				billingPeriodStart, err := time.Parse("20060102", parts[0])
				if err != nil {
					return false, errors.InternalError(err)
				}
				billingPeriodEnd, err := time.Parse("20060102", parts[1])
				if err != nil {
					return false, errors.InternalError(err)
				}
				if billingPeriodStart.After(prevCutoff) {
					// This billing report was not affected by change in retention as its start date
					// occurs after the previous cutoff
					continue
				} else if billingPeriodEnd.Before(prevCutoff) {
					// The entire time range of this billing report occurs before previous cutoff
					// and would have been expired by retention. Delete status in case we need to reprocess
					log.Printf("Billing report %s %s expired. Deleting status", ingStatus.BillingPeriod, ingStatus.AssemblyID)
					err = ctx.DeleteAssemblyIngestHistory(ingStatus.AssemblyID)
					if err != nil {
						return false, err
					}
				} else {
					// If we get here, this billing report occurs within the time frame of the previous cutoff
					// and it's data was partially expired. The report needs to be reingested
					log.Printf("Billing report %s %s partially expired. Deleting status", ingStatus.BillingPeriod, ingStatus.AssemblyID)
					err = ctx.DeleteAssemblyIngestHistory(ingStatus.AssemblyID)
					if err != nil {
						return false, err
					}
				}
			}
		}
		_, err := ctx.CostDB.Query("ALTER RETENTION POLICY \"%s\" ON \"%s\" DURATION %dd REPLICATION 1",
			ctx.retentionPolicyName, ctx.CostDB.databaseName, days)
		if err != nil {
			return false, err
		}
		return true, nil
	}
	return false, nil
}

// DropRetentionPolicy deletes the retention policy for this report
func (ctx *CostReportContext) DropRetentionPolicy() error {
	_, err := ctx.CostDB.Query("DROP RETENTION POLICY \"%s\" ON \"%s\"", ctx.retentionPolicyName, ctx.CostDB.databaseName)
	return err
}

// Close underlying client
func (db *CostDatabase) Close() {
	db.client.Close()
}
