// Copyright 2017 Applatix, Inc.
package routers

import (
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/applatix/claudia"
	"github.com/applatix/claudia/costdb"
	"github.com/applatix/claudia/errors"
	"github.com/applatix/claudia/parser"
	"github.com/applatix/claudia/server"
	"github.com/applatix/claudia/userdb"
	"github.com/applatix/claudia/util"
	"github.com/gorilla/mux"
	"github.com/influxdata/influxdb/models"
)

// writeReportHTTPCacheHeaders writes HTTP headers to enable client side caching
func writeReportHTTPCacheHeaders(report *userdb.Report, w http.ResponseWriter) {
	// See http://stackoverflow.com/questions/1046966/whats-the-difference-between-cache-control-max-age-0-and-no-cache
	w.Header().Set("Cache-Control", "max-age=0")
	w.Header().Set("ETag", report.ETag())
}

// checkCacheReuse responds to the client 304 NotModified if requestor supplied a up-to-date ETag. Returns true if cache is reusable
func checkCacheReuse(report *userdb.Report, r *http.Request, w http.ResponseWriter) bool {
	if report.Status == claudia.ReportStatusProcessing {
		return false
	}
	if etag, ok := r.Header["If-None-Match"]; ok && len(etag) > 0 && report.ETag() == etag[0] {
		w.WriteHeader(http.StatusNotModified)
		return true
	}
	return false
}

// rootDimensionHandler is a http handler to /v1/dimensions
func rootDimensionHandler(sc *server.ServerContext) func(http.ResponseWriter, *http.Request) {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		si, err := sc.SessionManager.ValidateSession(w, r)
		if err != nil {
			return
		}
		report, err := sc.GetDefaultReport(si.UserID)
		if util.ErrorHandler(err, w) != nil {
			return
		}
		if checkCacheReuse(report, r, w) {
			return
		}
		dimNames := []string{
			parser.ColumnUsageAccountID.APIName,
			parser.ColumnRegion.APIName,
			"resourcetags",
			parser.ColumnService.APIName,
		}
		items, err := dimensionHandlerHelper(sc, report, dimNames, w, r)
		if err != nil {
			return
		}
		writeReportHTTPCacheHeaders(report, w)
		util.SuccessHandler(items, w)
	})
}

// dimensionHandler is a http handler to /v1/dimensions/{dimension} (account, region, service, tags) and /v1/dimensions/{dimension}/{subdimension}
func dimensionHandler(sc *server.ServerContext) func(http.ResponseWriter, *http.Request) {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		si, err := sc.SessionManager.ValidateSession(w, r)
		if err != nil {
			return
		}
		vars := mux.Vars(r)
		dimensionAPIName := vars["dimension"]
		dimNames := []string{dimensionAPIName}
		report, err := sc.GetDefaultReport(si.UserID)
		if util.ErrorHandler(err, w) != nil {
			return
		}
		if checkCacheReuse(report, r, w) {
			return
		}
		items, err := dimensionHandlerHelper(sc, report, dimNames, w, r)
		if err != nil || items == nil {
			return
		}
		dimension := items[0]
		if subdimName, ok := vars["subdimension"]; ok {
			for _, subdim := range dimension.Dimensions {
				if subdim.Name == subdimName {
					util.SuccessHandler(subdim, w)
					return
				}
			}
			err := errors.Errorf(errors.CodeNotFound, "Dimension %s not found", subdimName)
			util.ErrorHandler(err, w)
		} else {
			writeReportHTTPCacheHeaders(report, w)
			util.SuccessHandler(dimension, w)
		}
	})
}

func dimensionHandlerHelper(sc *server.ServerContext, report *userdb.Report, dimensionNames []string, w http.ResponseWriter, r *http.Request) ([]*costdb.Dimension, error) {
	repCtx := sc.CostDB.NewCostReportContext(report.ID)
	filters, err := parseDimensionFiltersStrict(r.URL.Query())
	if util.ErrorHandler(err, w) != nil {
		return nil, err
	}
	items := make([]*costdb.Dimension, 0)
	for _, dimName := range dimensionNames {
		dimension, err := repCtx.GetDimension(dimName, filters)
		if util.ErrorHandler(err, w) != nil {
			return nil, err
		}
		aliases := sc.GetDisplayNameAliases(dimName, report)
		if aliases != nil {
			for _, subdim := range dimension.Dimensions {
				alias, exists := aliases[subdim.Name]
				if exists {
					subdim.DisplayName = alias
				}
			}
		}
		if dimName == parser.ColumnService.APIName {
			for _, dim := range dimension.Dimensions {
				for _, mktDim := range dim.Dimensions {
					switch mktDim.Name {
					case parser.ColumnProductCode.APIName, parser.ColumnDataTransferDest.APIName, parser.ColumnDataTransferSource.APIName:
						aliases = sc.GetDisplayNameAliases(mktDim.Name, report)
						if aliases != nil {
							for _, subdim := range mktDim.Dimensions {
								if alias, ok := aliases[subdim.Name]; ok {
									subdim.DisplayName = alias
								}
							}
						}
					}
				}
			}
		}
		items = append(items, dimension)
	}
	return items, nil
}

// costHandler is the http handler for /v1/cost
func costHandler(sc *server.ServerContext) func(http.ResponseWriter, *http.Request) {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		si, err := sc.SessionManager.ValidateSession(w, r)
		if err != nil {
			return
		}
		report, err := sc.GetDefaultReport(si.UserID)
		if util.ErrorHandler(err, w) != nil {
			return
		}
		if checkCacheReuse(report, r, w) {
			return
		}
		repCtx := sc.CostDB.NewCostReportContext(report.ID)
		costQuery, err := parseCostQueryParams(r.URL.Query())
		if util.ErrorHandler(err, w) != nil {
			return
		}
		rows, err := repCtx.Cost(costQuery)
		if util.ErrorHandler(err, w) != nil {
			return
		}
		transformRows(sc, report, costQuery, rows)
		writeReportHTTPCacheHeaders(report, w)
		util.SuccessHandler(rows, w)
	})
}

// transformRows will add dimension metadata to the cost data, such dimension name and display name (if available)
func transformRows(sc *server.ServerContext, report *userdb.Report, costQuery *costdb.CostQuery, rows []models.Row) {
	//repCtx := sc.CostDB.NewCostReportContext(report.ID)
	aliases := sc.GetDisplayNameAliases(costQuery.GroupBy, report)
	for _, row := range rows {
		if row.Tags == nil {
			continue
		}
		var seriesName string
		// The tag field from the influxdb result will be a mapping of the column name to column value
		// e.g. "tags": {"claudia/EC2InstanceType": "m3.2xlarge"}
		// We want to transform this to include: dimension API name, display name, and name
		// This enables some navigation elements
		for k, v := range row.Tags {
			seriesName = v
			delete(row.Tags, k) // remove the database column name from payload
			break
		}
		displayName := seriesName
		if aliases != nil {
			if alias, ok := aliases[seriesName]; ok {
				displayName = alias
			}
		}
		row.Tags["dimension"] = costQuery.GroupBy
		row.Tags["name"] = seriesName
		row.Tags["display_name"] = displayName
	}
}

// countHandler is the http handler for /v1/count
func countHandler(sc *server.ServerContext) func(http.ResponseWriter, *http.Request) {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		si, err := sc.SessionManager.ValidateSession(w, r)
		if err != nil {
			return
		}
		report, err := sc.GetDefaultReport(si.UserID)
		if util.ErrorHandler(err, w) != nil {
			return
		}
		if checkCacheReuse(report, r, w) {
			return
		}
		repCtx := sc.CostDB.NewCostReportContext(report.ID)
		costQuery, err := parseCostQueryParams(r.URL.Query())
		if util.ErrorHandler(err, w) != nil {
			return
		}
		costQuery.Aggregator = "COUNT(DISTINCT(\"%s\"))"
		costQuery.Field = parser.ColumnResourceID.ColumnName
		rows, err := repCtx.Cost(costQuery)
		if util.ErrorHandler(err, w) != nil {
			return
		}
		transformRows(sc, report, costQuery, rows)
		writeReportHTTPCacheHeaders(report, w)
		util.SuccessHandler(rows, w)
	})
}

// usageHandler is the http handler for /v1/usage/{service}/{metric}
func usageHandler(sc *server.ServerContext) func(http.ResponseWriter, *http.Request) {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		si, err := sc.SessionManager.ValidateSession(w, r)
		if err != nil {
			return
		}
		vars := mux.Vars(r)
		serviceName, ok := vars["service"]
		if !ok {
			err = errors.New(errors.CodeBadRequest, "Usage query must supply a service")
			util.ErrorHandler(err, w)
			return
		}
		report, err := sc.GetDefaultReport(si.UserID)
		if util.ErrorHandler(err, w) != nil {
			return
		}
		if checkCacheReuse(report, r, w) {
			return
		}
		costQuery, err := parseCostQueryParams(r.URL.Query())
		if util.ErrorHandler(err, w) != nil {
			return
		}
		costQuery.Field = parser.ColumnUsageAmount.ColumnName
		costQuery.Filters[parser.ColumnService.ColumnName] = []string{serviceName}

		repCtx := sc.CostDB.NewCostReportContext(report.ID)
		usageUnits, err := repCtx.GetUsageUnits(serviceName)
		if util.ErrorHandler(err, w) != nil {
			return
		}
		if len(usageUnits) > 1 {
			metricName, ok := vars["metric"]
			if !ok {
				validUnits := make([]string, len(usageUnits))
				for i, u := range usageUnits {
					validUnits[i] = u.Name
				}
				err = errors.Errorf(errors.CodeBadRequest, "Usage query of service '%s' is ambiguous. Choose from units: %s", serviceName, strings.Join(validUnits, ", "))
				util.ErrorHandler(err, w)
				return
			}
			var usageUnit *costdb.UsageUnit
			for _, u := range usageUnits {
				if u.Name == metricName {
					usageUnit = u
					break
				}
			}
			if usageUnit == nil {
				err = errors.Errorf(errors.CodeBadRequest, "Usage unit %s is not a valid metric of %s", metricName, serviceName)
				util.ErrorHandler(err, w)
				return
			}
			_, exists := costQuery.Filters[parser.ColumnUsageFamily.ColumnName]
			if !exists {
				// We must apply the usage family filter if user query did not supply it, in order for the usage query to make sense
				costQuery.Filters[parser.ColumnUsageFamily.ColumnName] = usageUnit.UsageFamilies
			}
		}
		rows, err := repCtx.Cost(costQuery)
		if util.ErrorHandler(err, w) != nil {
			return
		}
		transformRows(sc, report, costQuery, rows)
		writeReportHTTPCacheHeaders(report, w)
		util.SuccessHandler(rows, w)
	})
}

// Attempt multiple acceptable time formats
func parseTime(timeStr string) (time.Time, error) {
	t, err := time.Parse("2006-01-02", timeStr)
	if err != nil {
		return t, errors.Errorf(errors.CodeBadRequest, "Invalid time: %s", timeStr)
	}
	return t, nil
}

// parseDimensionFilters parses query args related to dimensions
func parseDimensionFilters(params url.Values) (map[string][]string, url.Values) {
	filters := make(map[string][]string)
	remaining := make(url.Values)
	for k, v := range params {
		val := v[0]
		columnName := parser.APINameToColumnName(k)
		if columnName == nil {
			remaining[k] = v
		} else {
			// NOTE: comma is acceptable as a delimiter because resouce tags cannot have commas
			// http://docs.aws.amazon.com/AWSEC2/latest/UserGuide/Using_Tags.html#tag-restrictions
			filters[*columnName] = strings.Split(val, ",")
		}
	}
	return filters, remaining
}

// parseDimensionFiltersStrict parses query args related to dimensions and returns error if any unrecognized dimensions
func parseDimensionFiltersStrict(params url.Values) (map[string][]string, error) {
	filters, remaining := parseDimensionFilters(params)
	if len(remaining) > 0 {
		keys := make([]string, len(remaining))
		i := 0
		for k := range remaining {
			keys[i] = k
			i++
		}
		err := errors.Errorf(errors.CodeBadRequest, "Unknown dimension(s): %s", keys)
		return nil, err
	}
	return filters, nil
}

func parseCostQueryParams(params url.Values) (*costdb.CostQuery, error) {
	costQuery := costdb.CostQuery{}
	var err error
	filters, remaining := parseDimensionFilters(params)
	costQuery.Filters = filters
	for k, v := range remaining {
		val := v[0]
		switch k {
		case "from":
			costQuery.From, err = parseTime(val)
		case "to":
			costQuery.To, err = parseTime(val)
		case "group_by":
			costQuery.GroupBy = val
		case "interval":
			// TODO: decide if we want to round down 'from' date if interval is weekly
			costQuery.Interval, err = costdb.ParseInterval(val)
		case "blended":
			val = strings.ToLower(val)
			if val == "true" || val == "1" || val == "t" {
				costQuery.Field = parser.ColumnBlendedCost.ColumnName
			}
		default:
			err = errors.Errorf(errors.CodeBadRequest, "Unknown param: %s", k)
		}
		if err != nil {
			return nil, err
		}
	}
	if costQuery.Interval != "" && costQuery.From.IsZero() && costQuery.To.IsZero() {
		err = errors.New(errors.CodeBadRequest, "Timeframe required when supplying interval")
		return nil, err
	}
	return &costQuery, nil
}
