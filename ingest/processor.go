// Copyright 2017 Applatix, Inc.
package ingest

import (
	"archive/zip"
	"bufio"
	"compress/gzip"
	"encoding/csv"
	"io"
	"log"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/applatix/claudia"
	"github.com/applatix/claudia/billingbucket"
	"github.com/applatix/claudia/costdb"
	"github.com/applatix/claudia/errors"
	"github.com/applatix/claudia/parser"
)

// GetCSVReader returns a CSV reader given a file
func GetCSVReader(file *os.File) (*csv.Reader, error) {
	var reader *csv.Reader
	if strings.HasSuffix(file.Name(), ".gz") {
		gzReader, err := gzip.NewReader(bufio.NewReader(file))
		if err != nil {
			return nil, errors.InternalError(err)
		}
		reader = csv.NewReader(gzReader)
	} else {
		reader = csv.NewReader(bufio.NewReader(file))
	}
	return reader, nil
}

// unzipReport extracts a .zip file in place.
// If user's reports are in zip format, we must first unzip the report instead of using a gzip reader stream
// as zip files cannot be streamed safely: https://github.com/golang/go/issues/10568
func unzipReport(zipPath string) (string, error) {
	if !strings.HasSuffix(zipPath, ".zip") {
		return "", errors.Errorf("%s does not have .zip extension: cannot formulate safe extraction path", zipPath)
	}
	r, err := zip.OpenReader(zipPath)
	if err != nil {
		return "", errors.InternalError(err)
	}
	defer func() {
		if err := r.Close(); err != nil {
			panic(err)
		}
	}()
	extractDir := path.Dir(zipPath)
	if len(r.File) != 1 {
		return "", errors.Errorf("Failed to unzip %s: zip archive had %d files (expected 1)", zipPath, len(r.File))
	}
	extractPath := filepath.Join(extractDir, r.File[0].Name)
	fileReader, err := r.File[0].Open()
	if err != nil {
		return "", errors.InternalError(err)
	}
	defer fileReader.Close()
	targetFile, err := os.OpenFile(extractPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, r.File[0].Mode())
	if err != nil {
		return "", errors.InternalError(err)
	}
	defer targetFile.Close()
	if _, err := io.Copy(targetFile, fileReader); err != nil {
		return "", errors.InternalError(err)
	}
	return extractPath, nil
}

// shouldIngest checks whether or not we should ingest the report based on if
// we have seen this manifest's billingPeriod/assemblyID before and it has been successfully processed
func shouldIngest(repCtx *costdb.CostReportContext, manifest *billingbucket.Manifest) bool {
	log.Printf("Determining if %s/%s/%s AssemblyID %s should be ingested", manifest.Bucket, manifest.ReportPath(), manifest.BillingPeriodString(), manifest.AssemblyID)
	ingStatusByBillingPeriod := func() (*costdb.IngestStatus, error) {
		return repCtx.CostDB.GetIngestStatusByBillingPeriod(manifest.Bucket, manifest.ReportPath(), manifest.BillingPeriodString())
	}
	ingStatusByAssemblyID := func() (*costdb.IngestStatus, error) {
		return repCtx.CostDB.GetIngestStatus(manifest.AssemblyID)
	}
	funcs := map[string]func() (*costdb.IngestStatus, error){
		"billing period": ingStatusByBillingPeriod,
		"assemblyID":     ingStatusByAssemblyID,
	}
	for method, getIngestStatusFunc := range funcs {
		ingestStatus, err := getIngestStatusFunc()
		if err != nil {
			log.Printf("Failed to retrieve ingest status by %s: %s", method, err)
			return false
		}
		if ingestStatus == nil {
			log.Printf("Ingest status by %s has never been processed", method)
			return true
		}
		if ingestStatus.ErrorMessage != "" {
			log.Printf("Previous ingest status by %s had error: %s", method, ingestStatus.ErrorMessage)
			return true
		}
		if ingestStatus.FinishTime == nil {
			log.Printf("Found interrupted ingest by %s. Proceeding with ingest.", method)
			return true
		}
		if ingestStatus.ParserVersion < parser.ParserVersion {
			log.Printf("Previous ingest was parsed with older parser version: %d. Current version: %d. Proceeding with ingest", ingestStatus.ParserVersion, parser.ParserVersion)
			return true
		}
	}
	log.Printf("Ingest of %s/%s/%s AssemblyID %s already finished. Skipping ingest", manifest.Bucket, manifest.ReportPath(), manifest.BillingPeriodString(), manifest.AssemblyID)
	return false
}

// IngestReportFile ingests a report file into the cost database
func IngestReportFile(repCtx *costdb.CostReportContext, job *manifestJob, reportPath string, run *bool) error {
	var err error
	log.Printf("Processing %s.\n", reportPath)
	if strings.HasSuffix(reportPath, ".zip") {
		reportPath, err = unzipReport(reportPath)
		if err != nil {
			return err
		}
	}
	file, err := os.Open(reportPath)
	if err != nil {
		return errors.InternalError(err)
	}
	defer file.Close()

	startRecords, _ := repCtx.CountRecords()
	reader, err := GetCSVReader(file)
	if err != nil {
		return err
	}
	fields, err := reader.Read()
	if err != nil {
		return errors.InternalError(err)
	}
	log.Printf("Fields determined to be %s", fields)

	// Create a new point batch
	bp, err := repCtx.NewBatchPoints()
	if err != nil {
		return err
	}
	var lineNum int64
	var wrote int64
	startTime := time.Now()
	var elapsed time.Duration
	billingPeriodStr := job.manifest.BillingPeriodString()

	for {
		if run != nil && !*run {
			break
		}
		line, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return errors.InternalError(err)
		}
		lineNum++

		lineItem, err := parser.ParseLine(fields, line)
		if err != nil {
			return err
		}
		if lineItem == nil {
			log.Printf("Report %s (%s) line %d skipped", repCtx.ReportID, reportPath, lineNum)
			continue
		}
		// Add the line number as a nanosecond offset to ensure data points are not deduped by InfluxDB
		lineItem.Timestamp = lineItem.Timestamp.Add(time.Duration(lineNum))
		// Add billing bucket and report path to the line item
		lineItem.Tags[parser.ColumnBillingBucket.ColumnName] = job.bucket.Bucketname
		lineItem.Tags[parser.ColumnBillingReportPath.ColumnName] = job.bucket.ReportPath
		lineItem.Tags[parser.ColumnBillingPeriod.ColumnName] = billingPeriodStr

		pt, err := repCtx.NewPoint(lineItem.Tags, lineItem.Fields, lineItem.Timestamp)
		if err != nil {
			return err
		}

		bp.AddPoint(pt)
		wrote++
		if lineNum%int64(claudia.IngestdBatchInterval) == 0 {
			// Write the batch and print some progress
			elapsed = time.Now().Sub(startTime)
			log.Printf("Report %s (%s) processed %d lines (%d points written) %f/s", repCtx.ReportID, reportPath, lineNum, wrote, float64(lineNum)/elapsed.Seconds())
			err = repCtx.CostDB.Write(bp)
			if err != nil {
				return err
			}
			bp, err = repCtx.NewBatchPoints()
			if err != nil {
				return err
			}
		}
	}
	err = repCtx.CostDB.Write(bp)
	if err != nil {
		return err
	}
	elapsed = time.Now().Sub(startTime)
	currRecords, _ := repCtx.CountRecords()
	created := currRecords - startRecords
	cardinality, _ := repCtx.SeriesCardinality()
	log.Printf("Report %s (%s) completed import in %fs (%f/s) (%d items) (wrote: %d, skipped: %d) New Records: %d (duplicated: %d) Cardinality: %d",
		repCtx.ReportID, reportPath, elapsed.Seconds(), float64(wrote)/elapsed.Seconds(), lineNum, wrote, lineNum-wrote, created, wrote-created, cardinality)
	return nil
}
