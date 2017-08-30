// Copyright 2017 Applatix, Inc.
package ingest

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/applatix/claudia"
	"github.com/applatix/claudia/billingbucket"
	"github.com/applatix/claudia/costdb"
	"github.com/applatix/claudia/errors"
	"github.com/applatix/claudia/parser"
	"github.com/applatix/claudia/userdb"
	"github.com/applatix/claudia/util"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/gorilla/handlers"
	"github.com/gorilla/mux"
)

// IngestSvcContext is the context wrapper around the ingestd service
type IngestSvcContext struct {
	userDB      *userdb.UserDatabase
	costDB      *costdb.CostDatabase
	reportDir   string
	updateCh    chan bool
	interruptCh chan bool
	numWorkers  int
}

// This regex will match a billing period, e.g. YYYYMMDD-YYYYMMDD
var billingPeriodMatcher = regexp.MustCompile("/?(\\d{8}-\\d{8})/?")

// NewIngestSvcContext returns a new ingestd service context
func NewIngestSvcContext(userDB *userdb.UserDatabase, costDbURL string, reportDir string, workers int) (*IngestSvcContext, error) {
	reportDir = path.Clean(reportDir)
	costDB, err := costdb.NewCostDatabase(costDbURL)
	if err != nil {
		return nil, errors.InternalError(err)
	}
	return &IngestSvcContext{userDB: userDB, costDB: costDB, reportDir: reportDir, numWorkers: workers}, nil
}

func reportMonitorHandler(isc *IngestSvcContext) func(http.ResponseWriter, *http.Request) {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Println("Notified of update")
		if isc.interruptCh != nil {
			log.Println("Interrupting process interval")
			isc.interruptCh <- true
		}
		isc.updateCh <- true
		util.SuccessHandler(nil, w)
	})
}

// poller is the background worker which will periodically check the buckets for new reports and process them
func (isc *IngestSvcContext) poller() {
	ticker := time.NewTicker(1 * time.Hour)
	defer ticker.Stop()
	retry := make(chan bool, 32)
	isc.updateCh <- true
	for {
		var err error
		select {
		case <-ticker.C:
			log.Println("Timer expired. Forcing update")
			err = isc.processInterval()
		case <-isc.updateCh:
			log.Println("Update received from update channel")
			err = isc.processInterval()
		case <-retry:
			log.Println("Reprocessing due to retry")
			err = isc.processInterval()
		}
		if err != nil {
			log.Printf("Process interval failed: %s", err)
			if apiErr, ok := err.(errors.APIError); ok {
				log.Printf("%+v", apiErr)
			}
			outstandingRetries := len(retry)
			if outstandingRetries == 0 {
				go func() {
					log.Printf("Retrying in 5m")
					time.Sleep(5 * time.Minute)
					retry <- true
				}()
			} else {
				log.Printf("Skipping queue of retry, %d outstanding retries", outstandingRetries)
			}
		} else {
			isc.cleanReportDir()
			log.Printf("Process interval completed successfully")
		}
	}
}

func (isc *IngestSvcContext) processInterval() error {
	log.Println("processInterval begin stats:")
	util.LogStats()
	defer func() {
		log.Println("processInterval end stats:")
		util.LogStats()
	}()
	isc.interruptCh = make(chan bool)
	defer func() {
		close(isc.interruptCh)
		isc.interruptCh = nil
	}()
	isc.purgeDeleted()
	log.Println("Checking for reports")
	tx, err := isc.userDB.Begin()
	if err != nil {
		return err
	}
	reports, err := tx.GetReports()
	if err != nil {
		tx.Rollback()
		return err
	}
	tx.Commit()
	toProcess, reportErrors := isc.generateJobs(reports)
	err = isc.processJobs(toProcess)
	// Update report statuses one last time. This is potentially redundant since we already call updateReportStatuses as jobs finish
	// But it is safe to do, and acts as a safety net in case it doesn't happen there.
	upStatusErr := isc.updateReportStatuses(reports, reportErrors)
	if upStatusErr != nil {
		log.Printf("Failed to update all report statuses: %s", upStatusErr)
	}
	if err != nil {
		return err
	}
	// Update report accounts one last time (with append=false) to delete
	// any accounts that no longer exist in any reports
	for _, rep := range reports {
		err = isc.updateReportAccounts(rep, false)
		if err != nil {
			log.Printf("Failed to update report %s accounts: %s", rep.ID, err)
		}
	}
	// AWS Marketplace information rarely ever changes. Only update marketplace information if we processed any jobs.
	// This effectively means we update once a day (whenever there are new billing reports)
	if len(toProcess) > 0 {
		err = isc.updateAWSMarketplaceInfo(reports)
		if err != nil {
			return err
		}
	}
	log.Printf("All %d manifests processed sucessfully", len(toProcess))
	return nil
}

// processJobs will iterate all manifests which need processing and block until all manifests have been completed. Returns first of any errors that occurred
func (isc *IngestSvcContext) processJobs(toProcess []*manifestJob) error {
	log.Printf("%d manifests need processing", len(toProcess))
	if len(toProcess) <= 0 {
		return nil
	}
	manifestCh := make(chan *manifestJob, len(toProcess))
	for _, job := range toProcess {
		manifestCh <- job
	}
	close(manifestCh)
	workers := make([]<-chan *manifestJob, isc.numWorkers)
	interruptChans := make([]chan bool, isc.numWorkers)
	for i := 0; i < isc.numWorkers; i++ {
		interruptChans[i] = make(chan bool)
		workers[i] = isc.manifestWorker(manifestCh, interruptChans[i])
	}
	// Start a goroutine which will listen on global interrupt channel and interrupt all workers
	go func() {
		for _ = range isc.interruptCh {
			log.Println("Interrupt received")
			break
		}
		for i, interruptCh := range interruptChans {
			log.Printf("Stopping worker %d", i)
			close(interruptCh)
		}
		interruptChans = nil
		log.Println("Interrupt channel poller exiting")
	}()

	completedCh := merge(workers...)
	var firstErr error
	errorCount := 0
	for job := range completedCh {
		err := isc.updateReportStatuses([]*userdb.Report{job.report}, nil)
		if err != nil {
			log.Printf("Failed to update report status: %s", err)
		}
		if job.err != nil {
			errorCount++
			if firstErr == nil {
				firstErr = job.err
			}
			if apiErr, ok := job.err.(errors.APIError); ok {
				log.Printf("%+v", apiErr)
			} else {
				log.Println(job.err)
			}
		}
	}
	if firstErr != nil {
		log.Printf("%d/%d manifests failed to process", errorCount, len(toProcess))
		return firstErr
	}
	return nil
}

// updateReportStatuses updates the status and status_detail fields of all reports
func (isc *IngestSvcContext) updateReportStatuses(reports []*userdb.Report, reportErrors map[string]error) error {
	log.Println("Updating report statuses")
	tx, err := isc.userDB.Begin()
	if err != nil {
		return err
	}
	for _, report := range reports {
		// If we encountered any errors when even generating jobs (e.g. credentials no longer valid),
		// we prefer those errors to be displayed, than any ingest errors
		if reportErrors != nil {
			if repErr, ok := reportErrors[report.ID]; ok {
				err = tx.UpdateUserReportStatus(report.ID, claudia.ReportStatusError, repErr.Error())
				if err != nil {
					tx.Rollback()
					return err
				}
				continue
			}
		}
		repCtx := isc.costDB.NewCostReportContext(report.ID)
		ingStatuses, err := repCtx.GetReportIngestStatuses()
		if err != nil {
			tx.Rollback()
			return err
		}
		reportStatus := claudia.ReportStatusCurrent
		statusDetail := ""
		processing := make([]string, 0)
		for _, is := range ingStatuses {
			rs, sd := is.StatusDetail()
			if rs == claudia.ReportStatusError {
				reportStatus = rs
				statusDetail = sd
				break
			} else if rs == claudia.ReportStatusProcessing {
				processing = append(processing, fmt.Sprintf("%s/%s/%s", is.Bucket, is.ReportPathPrefix, is.BillingPeriod))
				reportStatus = rs
			}
		}
		if reportStatus == claudia.ReportStatusProcessing {
			statusDetail = fmt.Sprintf("Processing: %s", strings.Join(processing, ", "))
		}
		err = tx.UpdateUserReportStatus(report.ID, reportStatus, statusDetail)
		if err != nil {
			tx.Rollback()
			return err
		}
	}
	return tx.Commit()
}

// purgeDeleted will purge any deleted reports and/or buckets from the report
func (isc *IngestSvcContext) purgeDeleted() error {
	log.Println("Purging any deleted reports and buckets")
	tx, err := isc.userDB.Begin()
	if err != nil {
		return err
	}
	reports, err := tx.GetReports()
	if err != nil {
		tx.Rollback()
		return err
	}
	tx.Commit()
	activeReportIDs := make(map[string]*userdb.Report)
	for _, report := range reports {
		activeReportIDs[report.ID] = report
	}
	reportIDs, err := isc.costDB.GetReportIDs()
	if err != nil {
		return err
	}
	deleted := 0
	for _, reportID := range reportIDs {
		report, active := activeReportIDs[reportID]
		repCtx := isc.costDB.NewCostReportContext(reportID)
		if !active {
			log.Printf("Report %s no longer active. Deleting report data", reportID)
			err = repCtx.DeleteReportData()
			if err != nil {
				return err
			}
			deleted++
			continue
		}
		buckets, err := repCtx.GetReportBuckets()
		if err != nil {
			return err
		}
		for _, b := range buckets {
			activeBucket := report.GetBucket(b.Bucket, b.ReportPathPrefix)
			if activeBucket == nil {
				log.Printf("Report %s bucket %s/%s no longer active. Deleting report data", report.ID, b.Bucket, b.ReportPathPrefix)
				err = repCtx.DeleteReportBillingBucketData(b.Bucket, b.ReportPathPrefix)
				if err != nil {
					return err
				}
				deleted++
			}
		}
		statuses, err := repCtx.GetReportIngestStatuses()
		if err != nil {
			return err
		}
		for _, is := range statuses {
			activeBucket := report.GetBucket(is.Bucket, is.ReportPathPrefix)
			if activeBucket == nil {
				log.Printf("Deleting ingest status report %s bucket %s/%s no longer active. Deleting status data", is.ReportID, is.Bucket, is.ReportPathPrefix)
				err = repCtx.DeleteBillingBucketHistory(is.Bucket, is.ReportPathPrefix)
				if err != nil {
					return err
				}
			}
		}
	}
	log.Printf("Finished checking for deleted reports. %d reports/buckets were deleted", deleted)
	return nil
}

// generateJobs will generate the list of manifest jobs needed to be processed, taking into consideration reports, buckets, retention, and ingest status
// Returns an array of jobs to process as well as any report/bucket level errors
func (isc *IngestSvcContext) generateJobs(reports []*userdb.Report) ([]*manifestJob, map[string]error) {
	toProcess := make([]*manifestJob, 0)
	reportErrors := make(map[string]error)
	for _, report := range reports {
		log.Printf("Processing report: %s", report.ID)
		for _, bucket := range report.Buckets {
			bucketJobs, err := isc.generateJobsFomBucket(report, bucket)
			if bucketJobs != nil {
				log.Printf("Bucket %s/%s generated (%d) jobs", bucket.Bucketname, bucket.ReportPrefix, len(bucketJobs))
				toProcess = append(toProcess, bucketJobs...)
			}
			if err != nil {
				log.Printf("Error generating jobs from bucket %s/%s: %s", bucket.Bucketname, bucket.ReportPrefix, err)
				if _, ok := reportErrors[report.ID]; !ok {
					reportErrors[report.ID] = err
				}
			}
		}
	}
	if len(reportErrors) == 0 {
		reportErrors = nil
	}
	return toProcess, reportErrors
}

// generateJobsFomBucket is a helper to generateJobs which generates the jobs specific to a bucket
func (isc *IngestSvcContext) generateJobsFomBucket(report *userdb.Report, bucket *userdb.Bucket) ([]*manifestJob, error) {
	log.Printf("Listing reports from bucket: %s/%s", bucket.Bucketname, bucket.ReportPrefix)
	billbuck, err := billingbucket.NewAWSBillingBucket(bucket.AWSAccessKeyID, bucket.AWSSecretAccessKey, bucket.Bucketname, bucket.Region, bucket.ReportPrefix)
	if err != nil {
		errMsg := fmt.Sprintf("Could not access billing bucket %s (reportPrefix: %s): %s", bucket.Bucketname, bucket.ReportPrefix, err)
		log.Printf(errMsg)
		return nil, err
	}
	manifestPaths, err := billbuck.GetManifestPaths()
	if err != nil {
		return nil, err
	}
	repCtx := isc.costDB.NewCostReportContext(report.ID)
	toProcess := make([]*manifestJob, 0)
	// Iterate reverse order so that the newer reports will be processed earlier
	// billbuck.GetManifestPaths() returns the items in lexographical order, which will be chronological
	for i := len(manifestPaths) - 1; i >= 0; i-- {
		manifestPath := manifestPaths[i]
		parts := billingPeriodMatcher.FindStringSubmatch(manifestPath)
		if len(parts) == 0 {
			return nil, errors.Errorf(errors.CodeInternal, "Unexpected report path location: %s", manifestPath)
		}
		billingPeriodEndStr := strings.Split(parts[len(parts)-1], "-")[1]
		billingPeriodEnd, err := time.Parse("20060102", billingPeriodEndStr)
		if err != nil {
			return nil, errors.InternalError(err)
		}
		reportAge := int(time.Now().Sub(billingPeriodEnd).Hours() / 24)
		if reportAge > report.RetentionDays {
			log.Printf("Skipping ingest of %s: billing period is outside retention (%dd > %dd)", manifestPath, reportAge, report.RetentionDays)
			continue
		}
		manifest, err := billbuck.GetManifest(manifestPath)
		if err != nil {
			return nil, err
		}
		if shouldIngest(repCtx, manifest) {
			toProcess = append(toProcess, &manifestJob{report, bucket, billbuck, manifest, nil})
		}
	}
	return toProcess, nil
}

// merges multiple completed manifest job channels into one channel
func merge(cs ...<-chan *manifestJob) <-chan *manifestJob {
	var wg sync.WaitGroup
	out := make(chan *manifestJob)

	// Start an output goroutine for each input channel in cs.  output
	// copies values from c to out until c is closed, then calls wg.Done.
	output := func(c <-chan *manifestJob) {
		for j := range c {
			out <- j
		}
		defer wg.Done()
	}
	wg.Add(len(cs))
	for _, c := range cs {
		go output(c)
	}

	// Start a goroutine to close out once all the output goroutines are done.
	// This must start after the wg.Add call.
	go func() {
		wg.Wait()
		close(out)
	}()
	return out
}

// manifestJob encapsulates work needed to be done against a manifest
type manifestJob struct {
	report   *userdb.Report
	bucket   *userdb.Bucket
	billbuck *billingbucket.AWSBillingBucket
	manifest *billingbucket.Manifest
	err      error
}

// manifestWorker will spawn a goroutine worker listening on the manifest channel to immediately begin work on the jobs in the queue.
// Workers can be interrupted by sending a boolean (of any value) to the given interrupt channel.
func (isc *IngestSvcContext) manifestWorker(manifestCh chan *manifestJob, interruptCh chan bool) <-chan *manifestJob {
	completedCh := make(chan *manifestJob)
	// The run variable is passed up the call stack to the ingest loop, which will periodically checks if it should continue ingesting.
	// This flag makes it possible to interrupt the ingest loop.
	run := true
	go func() {
		log.Println("Manifest worker starting")
		for job := range manifestCh {
			if !run {
				break
			}
			job.err = isc.doJob(job, &run)
			// Update the mtime of the report regardless of success or error since there will be new data
			tx, err := isc.userDB.Begin()
			if err != nil {
				log.Println("Failed to update report mtime: Could not start transaction")
			} else {
				err = isc.updateReportAccounts(job.report, true)
				if err != nil {
					log.Printf("Failed to update report %s accounts: %s", job.report.ID, err)
				}
				err = tx.UpdateUserReportMtime(job.report.ID)
				if err != nil {
					log.Printf("Failed to update report %s mtime: %s", job.report.ID, err)
					tx.Rollback()
				}
				tx.Commit()
			}
			completedCh <- job
		}
		close(completedCh)
		log.Println("Manifest worker exiting")
	}()
	// Start a goroutine which will listen on this worker's interrupt channel, and set the worker's run sentinal to be false upon interrupt
	go func() {
		for _ = range interruptCh {
			break
		}
		run = false
	}()
	return completedCh
}

// doJob will perform the actual logic of processing a manifest job
func (isc *IngestSvcContext) doJob(job *manifestJob, run *bool) error {
	// Now that we have decided that this manifest and its reports should be ingested, we should always purge any
	// existing data from the same billing period and billing bucket. This covers the cases where we have:
	// 1) a new billing report in an unfinalized billing month where previous data is invalid
	// 2) a interrupted ingest where there is partially ingested data in the database for the same billing period we are about to ingest
	repCtx := isc.costDB.NewCostReportContext(job.report.ID)
	err := repCtx.RecordIngestStart(*job.manifest)
	if err != nil {
		return err
	}
	// Updates the report status to processing
	err = isc.updateReportStatuses([]*userdb.Report{job.report}, nil)
	if err != nil {
		log.Printf("Failed to update report status of %s: %s", job.report.ID, err)
	}
	// First iteration is used as a flag to indicate if we should purge the billing period data.
	// We only want to do this once, and only if at least one download was successful, so that if
	// credentials suddenly become invalid, we don't delete a months worth of data and left unable
	// to process more data.
	firstIteration := true
	err = nil
	for _, reportKey := range job.manifest.ReportKeys {
		if !*run {
			break
		}
		localPath := fmt.Sprintf("%s/%s/%s/%s/%s", isc.reportDir, job.bucket.ID, job.manifest.BillingPeriodString(), job.manifest.AssemblyID, path.Base(reportKey))
		log.Printf("Downloading %s to: %s", reportKey, localPath)
		err = job.billbuck.Download(reportKey, localPath, false)
		if err != nil {
			log.Printf("Failed to download %s: %s", reportKey, err)
			break
		}
		if firstIteration {
			err := repCtx.PurgeBillingPeriodSeries(job.billbuck.Bucket, job.billbuck.ReportPathPrefix, job.manifest.BillingPeriodString())
			if err != nil {
				// If we can't purge previous billing series, do not continue with processing additional report keys.
				// Otherwise, we will double count the data (from previous ingest) and the cost/usage will be over stated.
				return err
			}
			firstIteration = false
		}
		err = IngestReportFile(repCtx, job, localPath, run)
		if err != nil {
			errMsg := fmt.Sprintf("Failed to ingest %s: %s", localPath, err)
			log.Printf(errMsg)
			// Ignore any failures to record ingest error since we want original to perculate
			_ = repCtx.RecordIngestError(*job.manifest, errMsg)
			break
		}
		log.Printf("Deleting processed report path: %s", localPath)
		err = os.Remove(localPath)
		if err != nil {
			err = errors.InternalError(err)
			break
		}
	}
	if err != nil {
		b, _ := json.MarshalIndent(job.manifest, "", "  ")
		log.Printf("Error (%s) occurred processing manifest:\n%s", err, string(b))
		return err
	}
	if !*run {
		// Do not record a finish time. This will result in the report status remaining in "processing" status
		log.Printf("Ingest interrupted during ingestion of report %s %s/%s/%s",
			job.report.ID, job.bucket.Bucketname, job.bucket.ReportPrefix, job.manifest.BillingPeriodString())
	} else {
		err = repCtx.RecordIngestFinish(*job.manifest)
	}
	return err
}

// cleanReportDir cleans remnant working directories in the report dir (anything that looks like a report UUID)
func (isc *IngestSvcContext) cleanReportDir() error {
	workDirPaths, err := filepath.Glob(fmt.Sprintf("%s/*", isc.reportDir))
	if err != nil {
		return errors.InternalError(err)
	}
	var firstError error
	for _, workDir := range workDirPaths {
		reportID := path.Base(workDir)
		if util.IsUUIDv4(reportID) {
			log.Printf("Removing %s", workDir)
			err = os.RemoveAll(workDir)
			if err != nil {
				err = errors.InternalError(err)
				log.Printf("Failed to remove %s: %s", workDir, err)
				if firstError == nil {
					firstError = err
				}
			}
		}
	}
	return firstError
}

// updateReportAccounts will update the user database with any new usage accounts found in cost database
func (isc *IngestSvcContext) updateReportAccounts(report *userdb.Report, append bool) error {
	rctx := isc.costDB.NewCostReportContext(report.ID)
	accountIDs, err := rctx.TagValues(parser.ColumnUsageAccountID, nil)
	if err != nil {
		return err
	}
	// Discover and add any newly discovered accounts from the billing report
	for _, accountID := range accountIDs {
		exists := false
		for _, account := range report.Accounts {
			if account.AWSAccountID == accountID {
				exists = true
				break
			}
		}
		if !exists {
			tx, err := isc.userDB.Begin()
			if err != nil {
				return err
			}
			err = tx.AddReportAccount(report.ID, accountID)
			if err != nil {
				tx.Rollback()
				return err
			}
			tx.Commit()
		}
	}
	if !append {
		return isc.purgeDeletedAccounts(report, accountIDs)
	}
	return nil
}

// purgeDeletedAccounts deletes accountIDs from the userdb if the accounts no longer exist in the reports
func (isc *IngestSvcContext) purgeDeletedAccounts(report *userdb.Report, accountIDs []string) error {
	tx, err := isc.userDB.Begin()
	if err != nil {
		return err
	}
	report, err = tx.GetUserReport(report.OwnerUserID, report.ID)
	if err != nil {
		tx.Rollback()
		return err
	}
	if report == nil {
		log.Printf("Report %s no longer exists", report.ID)
		tx.Rollback()
		return nil
	}
	// If report is in a good state (no errors, and no unfinished ingests), then it is safe to unassociate accounts
	if report.Status != claudia.ReportStatusCurrent {
		log.Printf("Skipping account deletion. Report %s in '%s' status", report.ID, report.Status)
		tx.Rollback()
		return nil
	}
	// Delete accounts from the report that no longer show up in the reports (e.g. a bucket was deleted)
	for _, account := range report.Accounts {
		exists := false
		for _, accountID := range accountIDs {
			if account.AWSAccountID == accountID {
				exists = true
				break
			}
		}
		if !exists {
			err = tx.DeleteReportAccount(report.ID, account.AWSAccountID)
			if err != nil {
				tx.Rollback()
				return err
			}
		}
	}
	tx.Commit()
	return nil
}

// updateAWSMarketplaceInfo will update the table mapping product codes to the display name
func (isc *IngestSvcContext) updateAWSMarketplaceInfo(reports []*userdb.Report) error {
	log.Println("Updating products")
	filters := make(map[string][]string)
	filters[parser.ColumnService.ColumnName] = []string{claudia.ServiceAWSMarketplace}
	for _, report := range reports {
		if len(report.Buckets) == 0 {
			log.Printf("Unable to use credentials from report %s to identify products: No buckets configured", report.ID)
			continue
		}
		rctx := isc.costDB.NewCostReportContext(report.ID)
		costDBProductCodes, err := rctx.TagValues(parser.ColumnProductCode, filters)
		if err != nil {
			return err
		}
		tx, err := isc.userDB.Begin()
		if err != nil {
			return err
		}
		for _, productCode := range costDBProductCodes {
			product, err := identifyProduct(productCode, report.Buckets[0].AWSAccessKeyID, report.Buckets[0].AWSSecretAccessKey)
			if err != nil {
				log.Printf("Failed to identify product %s: %s", productCode, err)
				continue
			}
			if product != nil {
				if err := tx.UpsertProduct(product); err != nil {
					tx.Rollback()
					return err
				}
			}
		}
		if len(costDBProductCodes) > 0 {
			err = tx.UpdateUserReportMtime(report.ID) // Best effort
			if err != nil {
				log.Printf("Failed to update user report mtime: %s", err)
			}
		}
		tx.Commit()
	}
	return nil
}

// identifyProduct looks up a product code in AWS marketplace
func identifyProduct(productCode, awsAccessKeyID, awsSecretAccessKey string) (*userdb.AWSProductInfo, error) {
	log.Printf("Identifying product %s", productCode)
	cfg := aws.NewConfig()
	creds := credentials.NewStaticCredentials(awsAccessKeyID, awsSecretAccessKey, "")
	cfg = cfg.WithCredentials(creds)
	//cfg.WithLogLevel(aws.LogDebugWithRequestErrors)
	if cfg.Region == nil {
		cfg = cfg.WithRegion("us-east-1")
	}
	sess, err := session.NewSession(cfg)
	if err != nil {
		return nil, errors.InternalErrorWithMessage(err, "failed to create session")
	}
	ec2client := ec2.New(sess)

	describeReq := ec2.DescribeImagesInput{
		//DryRun: aws.Bool(true),
		Filters: []*ec2.Filter{
			&ec2.Filter{Name: aws.String("owner-id"), Values: aws.StringSlice([]string{claudia.AWSMarketplaceAccountID})},
			&ec2.Filter{Name: aws.String("product-code"), Values: []*string{&productCode}},
		},
	}
	res, err := ec2client.DescribeImages(&describeReq)
	if err != nil {
		return nil, errors.InternalError(err)
	}
	if len(res.Images) <= 0 {
		log.Printf("Lookup of product %s returned no results", productCode)
		return nil, nil
	}
	log.Printf("Product %s identified:\n%s", productCode, res)
	// If there are multiple images associated with the product code, we don't know which is the real image that is in use.
	// Just pick the first image in the list (which should be the latest version).
	image := res.Images[0]
	product := userdb.AWSProductInfo{ProductCode: productCode, Name: *image.Name, Description: *image.Description}
	return &product, nil
}

// Run is the main function of the ingestd service. It spawns a background goroutine
// to periodically check for updates and then has an API endpoint to be notified of
// new updates (such as a new report being added, or a change in buckets/credentials)
func (isc *IngestSvcContext) Run(port int) error {
	isc.userDB.Wait()
	isc.costDB.Wait()
	err := isc.costDB.CreateDatabase()
	if err != nil {
		return err
	}
	isc.updateCh = make(chan bool, 64)
	go isc.poller()

	r := mux.NewRouter()
	r.HandleFunc("/v1/refresh", reportMonitorHandler(isc)).Methods("POST")
	handler := handlers.LoggingHandler(os.Stdout, r)
	log.Println(fmt.Sprintf("Starting server on port %d", port))
	err = http.ListenAndServe(fmt.Sprintf(":%d", port), handler)
	return err
}
