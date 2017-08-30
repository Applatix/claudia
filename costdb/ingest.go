// Copyright 2017 Applatix, Inc.
package costdb

import (
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/applatix/claudia"
	"github.com/applatix/claudia/billingbucket"
	"github.com/applatix/claudia/errors"
	"github.com/applatix/claudia/parser"
	client "github.com/influxdata/influxdb/client/v2"
	"github.com/influxdata/influxdb/models"
)

// IngestStatus is the ingest status of a Manifest assembly
type IngestStatus struct {
	ReportID         string     `json:"report_id"`
	AssemblyID       string     `json:"assembly_id"`
	Bucket           string     `json:"bucket"`
	ReportPathPrefix string     `json:"report_path_prefix"`
	BillingPeriod    string     `json:"billing_period"`
	ErrorMessage     string     `json:"error,omitempty"`
	ParserVersion    int        `json:"parser_version"`
	StartTime        *time.Time `json:"start_time"`
	FinishTime       *time.Time `json:"finish_time"`
}

// Ingest events
const (
	EventIngestStart    = "STARTED"
	EventIngestFinished = "FINISHED"
	EventIngestError    = "ERROR"
)

// NewPoint returns a InfluxDB point ready to be stored in this costDB's measurement
func (ctx *CostReportContext) NewPoint(tags map[string]string, fields map[string]interface{}, t ...time.Time) (*client.Point, error) {
	return client.NewPoint(ctx.measurementName, tags, fields, t...)
}

// StatusDetail returns status and detail which can be used as a report status
func (is *IngestStatus) StatusDetail() (claudia.ReportStatus, string) {
	if is.ErrorMessage != "" {
		return claudia.ReportStatusError, is.ErrorMessage
	}
	if is.FinishTime == nil {
		return claudia.ReportStatusProcessing, ""
	}
	return claudia.ReportStatusCurrent, ""
}

// GetIngestStatus returns the ingest status of an Manifest assembly
func (db *CostDatabase) GetIngestStatus(assemblyID string) (*IngestStatus, error) {
	query := fmt.Sprintf("SELECT * FROM %s WHERE \"assemblyId\"='%s'", claudia.IngestStatusMeasurementName, assemblyID)
	results, err := db.Query(query)
	if err != nil {
		return nil, err
	}
	if len(results[0].Series) == 0 {
		return nil, nil
	}
	series := results[0].Series[0]
	ingStatus, err := parseIngestSeries(series)
	if err != nil {
		return nil, err
	}
	return ingStatus, nil
}

// GetReportIDs returns all report IDs known by the cost database
func (db *CostDatabase) GetReportIDs() ([]string, error) {
	results, err := db.Query("SHOW MEASUREMENTS WITH MEASUREMENT =~ /report_.*/")
	if err != nil {
		return nil, err
	}
	reportIDMap := make(map[string]bool)
	for _, s := range results[0].Series {
		measurementName := s.Values[0][0].(string)
		reportID := strings.SplitN(measurementName, "_", 2)[1]
		reportIDMap[reportID] = true
	}
	results, err = db.Query("SHOW TAG VALUES FROM \"%s\" WITH KEY = \"reportId\"", claudia.IngestStatusMeasurementName)
	if err != nil {
		return nil, err
	}
	for _, s := range results[0].Series {
		reportID := s.Values[0][1].(string)
		reportIDMap[reportID] = true
	}
	reportIDs := make([]string, 0)
	for reportID := range reportIDMap {
		reportIDs = append(reportIDs, reportID)
	}
	return reportIDs, nil
}

// GetIngestStatusByBillingPeriod returns the ingest status of a bucket and billing period
func (db *CostDatabase) GetIngestStatusByBillingPeriod(bucket, reportPathPrefix, billingPeriod string) (*IngestStatus, error) {
	results, err := db.Query("SELECT * FROM \"%s\" WHERE \"bucket\"='%s' AND \"reportPathPrefix\"='%s' AND \"billingPeriod\"='%s'",
		claudia.IngestStatusMeasurementName, bucket, escapeSingleQuote(reportPathPrefix), billingPeriod)
	if err != nil {
		return nil, err
	}
	if len(results) == 0 {
		return nil, nil
	}
	if len(results[0].Series) == 0 {
		return nil, nil
	}
	series := results[0].Series[0]
	ingStatus, err := parseIngestSeries(series)
	if err != nil {
		return nil, err
	}
	return ingStatus, nil
}

func parseIngestSeries(series models.Row) (*IngestStatus, error) {
	var err error
	ingStatus := IngestStatus{}
	for _, dataPoint := range series.Values {
		var dpTime time.Time
		for i, colVal := range dataPoint {
			columnName := series.Columns[i]
			switch columnName {
			case "time":
				dpTime, err = time.Parse("2006-01-02T15:04:05.999999999Z", colVal.(string))
				if err != nil {
					return nil, errors.InternalErrorf(err, "Failed to parse time: %s", colVal.(string))
				}
			case "reportId":
				ingStatus.ReportID = colVal.(string)
			case "bucket":
				ingStatus.Bucket = colVal.(string)
			case "reportPathPrefix":
				ingStatus.ReportPathPrefix = colVal.(string)
			case "billingPeriod":
				ingStatus.BillingPeriod = colVal.(string)
			case "assemblyId":
				ingStatus.AssemblyID = colVal.(string)
			case "parserVersion":
				parserVersion, err := colVal.(json.Number).Int64()
				if err != nil {
					return nil, errors.InternalError(err)
				}
				ingStatus.ParserVersion = int(parserVersion)
			case "error":
				if colVal != nil {
					ingStatus.ErrorMessage = colVal.(string)
				}
			case "event":
				status := colVal.(string)
				switch status {
				case EventIngestStart:
					ingStatus.StartTime = &dpTime
				case EventIngestFinished:
					ingStatus.FinishTime = &dpTime
				}
			}
		}
	}
	return &ingStatus, nil
}

// GetReportBuckets all buckets associated with the report
func (ctx *CostReportContext) GetReportBuckets() ([]*billingbucket.AWSBillingBucket, error) {
	bucketNames, err := ctx.TagValues(parser.ColumnBillingBucket, nil)
	if err != nil {
		return nil, err
	}
	buckets := make([]*billingbucket.AWSBillingBucket, 0)
	for _, bucketName := range bucketNames {
		filters := map[string][]string{
			parser.ColumnBillingBucket.ColumnName: []string{bucketName},
		}
		prefixes, err := ctx.TagValues(parser.ColumnBillingReportPathPrefix, filters)
		if err != nil {
			return nil, err
		}
		for _, prefix := range prefixes {
			billbuck := billingbucket.AWSBillingBucket{Bucket: bucketName, ReportPathPrefix: prefix}
			buckets = append(buckets, &billbuck)
		}
	}
	return buckets, nil
}

// GetReportIngestStatuses returns the ingest status of the report (e.g. if there are any errors)
func (ctx *CostReportContext) GetReportIngestStatuses() ([]*IngestStatus, error) {
	query := fmt.Sprintf("SELECT * FROM %s WHERE \"reportId\" = '%s' GROUP BY \"assemblyId\"", claudia.IngestStatusMeasurementName, ctx.ReportID)
	results, err := ctx.CostDB.Query(query)
	if err != nil {
		return nil, err
	}
	ingStatuses := make([]*IngestStatus, len(results[0].Series))
	for i, series := range results[0].Series {
		ingStatus, err := parseIngestSeries(series)
		if err != nil {
			return nil, err
		}
		// This assignment is necessary because InfluxDB will omit assemblyId from columns since we are grouping by it
		var assemblyID string
		for _, aid := range series.Tags {
			assemblyID = aid
		}
		ingStatus.AssemblyID = assemblyID
		ingStatuses[i] = ingStatus
	}
	return ingStatuses, nil

}

// DeleteAllIngestHistory deletes all ingest history for this report
func (ctx *CostReportContext) DeleteAllIngestHistory() error {
	log.Printf("Deleting ingest history for %s", ctx.ReportID)
	query := fmt.Sprintf("DROP SERIES FROM %s WHERE \"reportId\" = '%s'", claudia.IngestStatusMeasurementName, ctx.ReportID)
	_, err := ctx.CostDB.Query(query)
	return err
}

// DeleteBillingBucketHistory deletes all ingest history for the billing bucket & report prefix
// Called when a report bucket has been deleted
func (ctx *CostReportContext) DeleteBillingBucketHistory(bucketname, reportPrefix string) error {
	log.Printf("Deleting ingest history for reportID %s bucket %s reportPathPrefix %s", ctx.ReportID, bucketname, reportPrefix)
	query := fmt.Sprintf("DROP SERIES FROM %s WHERE \"reportId\" = '%s' AND \"bucket\" = '%s' AND \"reportPathPrefix\" = '%s'",
		claudia.IngestStatusMeasurementName, ctx.ReportID, bucketname, escapeSingleQuote(reportPrefix))
	_, err := ctx.CostDB.Query(query)
	return err
}

// DeleteAssemblyIngestHistory deletes ingest history for a given assembly id
// Called when updating a retention policy results in the need for reprocessing of report data
func (ctx *CostReportContext) DeleteAssemblyIngestHistory(assemblyID string) error {
	log.Printf("Deleting ingest history for reportID %s assemblyID %s", ctx.ReportID, assemblyID)
	query := fmt.Sprintf("DROP SERIES FROM %s WHERE \"reportId\" = '%s' AND \"assemblyId\" = '%s'",
		claudia.IngestStatusMeasurementName, ctx.ReportID, assemblyID)
	_, err := ctx.CostDB.Query(query)
	return err
}

// DeleteBillingPeriodIngestHistory deletes all ingest history for the billing bucket, report prefix, and billing period of this manifest
// Called right before we process a monthly report (to ensure data is not counted twice when processing cumulative reports)
func (ctx *CostReportContext) deleteBillingPeriodIngestHistory(manifest *billingbucket.Manifest) error {
	log.Printf("Deleting ingest history for reportID %s bucket %s reportPathPrefix %s billingPeriod: %s",
		ctx.ReportID, manifest.Bucket, manifest.ReportPathPrefix(), manifest.BillingPeriodString())
	query := fmt.Sprintf("DROP SERIES FROM %s WHERE \"reportId\" = '%s' AND \"bucket\" = '%s' AND \"reportPathPrefix\" = '%s' AND \"billingPeriod\" = '%s'",
		claudia.IngestStatusMeasurementName, ctx.ReportID, manifest.Bucket, escapeSingleQuote(manifest.ReportPathPrefix()), manifest.BillingPeriodString())
	_, err := ctx.CostDB.Query(query)
	return err
}

// RecordIngestStart will record in the database the start of a processing in the report
func (ctx *CostReportContext) RecordIngestStart(manifest billingbucket.Manifest) error {
	log.Println("Starting ingest")
	err := ctx.deleteBillingPeriodIngestHistory(&manifest)
	if err != nil {
		return err
	}
	return ctx.recordIngestHelper(manifest, EventIngestStart, "")
}

// RecordIngestFinish will record in the database the completion of a processing in the report
func (ctx *CostReportContext) RecordIngestFinish(manifest billingbucket.Manifest) error {
	log.Println("Finished ingest")
	return ctx.recordIngestHelper(manifest, EventIngestFinished, "")
}

// RecordIngestError will record in the database an error processing a report
func (ctx *CostReportContext) RecordIngestError(manifest billingbucket.Manifest, errorMsg string) error {
	log.Printf("Ingest errored with: %s", errorMsg)
	return ctx.recordIngestHelper(manifest, EventIngestError, errorMsg)
}

func (ctx *CostReportContext) recordIngestHelper(manifest billingbucket.Manifest, event string, errorMsg string) error {
	tags := map[string]string{
		"reportId":         ctx.ReportID,
		"assemblyId":       manifest.AssemblyID,
		"bucket":           manifest.Bucket,
		"reportPathPrefix": manifest.ReportPathPrefix(),
		"billingPeriod":    manifest.BillingPeriodString(),
	}
	fields := map[string]interface{}{
		"reportName":    manifest.ReportName,
		"parserVersion": parser.ParserVersion,
		"event":         event,
	}
	if errorMsg != "" {
		fields["error"] = errorMsg
	}
	bp, err := ctx.NewBatchPoints()
	// Don't use the reports retention policy so that this data will never expire
	bp.SetRetentionPolicy("")
	if err != nil {
		return err
	}
	pt, err := client.NewPoint(claudia.IngestStatusMeasurementName, tags, fields)
	if err != nil {
		return errors.InternalError(err)
	}
	bp.AddPoint(pt)
	err = ctx.CostDB.client.Write(bp)
	return errors.InternalError(err)
}

// escapeSingleQuote returns a string with an escaping single quote
func escapeSingleQuote(str string) string {
	return strings.Replace(str, "'", "\\'", -1)
}
