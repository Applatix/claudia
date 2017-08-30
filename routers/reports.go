// Copyright 2017 Applatix, Inc.
package routers

import (
	"encoding/json"
	"log"
	"net/http"
	"strings"

	"github.com/applatix/claudia/billingbucket"
	"github.com/applatix/claudia/errors"
	"github.com/applatix/claudia/server"
	"github.com/applatix/claudia/userdb"
	"github.com/applatix/claudia/util"
	"github.com/gorilla/mux"
)

// reportsHandler is the hadndler for /v1/reports
func reportsHandler(sc *server.ServerContext) func(http.ResponseWriter, *http.Request) {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		si, err := sc.SessionManager.ValidateSession(w, r)
		if err != nil {
			return
		}
		switch r.Method {
		case "GET":
			tx, err := sc.UserDB.Begin()
			if util.TXErrorHandler(err, tx, w) != nil {
				return
			}
			reports, err := tx.GetUserReports(si.UserID)
			if util.TXErrorHandler(err, tx, w) != nil {
				return
			}
			tx.Commit()
			for _, r := range reports {
				hideAWSSecretAccessKeys(r)
			}
			util.SuccessHandler(reports, w)
		case "POST":
			decoder := json.NewDecoder(r.Body)
			report := userdb.Report{}
			err = decoder.Decode(&report)
			err = errors.Wrap(err, errors.CodeBadRequest, "Invalid report")
			if util.ErrorHandler(err, w) != nil {
				return
			}
			tx, err := sc.UserDB.Begin()
			if util.TXErrorHandler(err, tx, w) != nil {
				return
			}
			reportID, err := tx.CreateUserReport(si.UserID, &report)
			if util.TXErrorHandler(err, tx, w) != nil {
				return
			}
			createdReport, err := tx.GetUserReport(si.UserID, reportID)
			if util.TXErrorHandler(err, tx, w) != nil {
				return
			}
			repCtx := sc.CostDB.NewCostReportContext(createdReport.ID)
			err = repCtx.CreateRetentionPolicy(createdReport.RetentionDays)
			if util.TXErrorHandler(err, tx, w) != nil {
				return
			}
			err = tx.Commit()
			if util.TXErrorHandler(err, tx, w) != nil {
				return
			}
			hideAWSSecretAccessKeys(createdReport)
			util.SuccessHandler(createdReport, w)
		default:
			util.ErrorHandler(errors.Errorf(errors.CodeBadRequest, "Unsupported method %s", r.Method), w)
		}
	})
}

// maskAccessKey returns the access key masked, only showing first two and last two characters.
func maskAccessKey(awsAccessKeyID string) string {
	maskedStr := make([]byte, len(awsAccessKeyID))
	for i := range maskedStr {
		if (i < 2 || i > len(awsAccessKeyID)-3) && len(awsAccessKeyID) > 6 {
			maskedStr[i] = awsAccessKeyID[i]
		} else {
			maskedStr[i] = '*'
		}
	}
	return string(maskedStr)
}

// validateBucket will verify the bucket and credentials are valid and sets its region
func validateBucket(bucket *userdb.Bucket) error {
	maskedAccessKey := maskAccessKey(bucket.AWSSecretAccessKey)
	log.Printf("Validating Bucket: %s/%s, AccessKeyID: %s, SecretAccessKey: %s", bucket.Bucketname, bucket.ReportPath, bucket.AWSAccessKeyID, maskedAccessKey)
	region, err := billingbucket.GetBucketRegion(bucket.Bucketname)
	if err != nil {
		return err
	}
	bucket.Region = region
	billbuck, err := billingbucket.NewAWSBillingBucket(bucket.AWSAccessKeyID, bucket.AWSSecretAccessKey, bucket.Bucketname, bucket.Region, bucket.ReportPath)
	if err != nil {
		log.Printf("Could not create billing bucket: %s", err)
		return err
	}
	dirContents, err := billbuck.ListDir(bucket.ReportPath)
	if err != nil {
		log.Printf("Failed to list billing bucket: %s", err)
		return err
	}
	if len(dirContents) == 0 {
		return errors.Errorf(errors.CodeBadRequest, "Bucket report path %s/%s empty or does not exist", bucket.Bucketname, bucket.ReportPath)
	}
	return nil
}

// reportHandler is the handler for /v1/reports/{reportID}
func reportHandler(sc *server.ServerContext) func(http.ResponseWriter, *http.Request) {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		si, err := sc.SessionManager.ValidateSession(w, r)
		if err != nil {
			return
		}
		vars := mux.Vars(r)
		reportID := vars["reportID"]
		switch r.Method {
		case "GET":
			tx, err := sc.UserDB.Begin()
			if util.TXErrorHandler(err, tx, w) != nil {
				return
			}
			report, err := tx.GetUserReport(si.UserID, reportID)
			if util.TXErrorHandler(err, tx, w) != nil {
				return
			}
			tx.Commit()
			hideAWSSecretAccessKeys(report)
			util.SuccessHandler(report, w)
		case "PUT":
			decoder := json.NewDecoder(r.Body)
			report := userdb.Report{}
			err = decoder.Decode(&report)
			if err != nil {
				err = errors.New(errors.CodeBadRequest, "Invalid report JSON")
			}
			if util.ErrorHandler(err, w) != nil {
				return
			}
			tx, err := sc.UserDB.Begin()
			if util.TXErrorHandler(err, tx, w) != nil {
				return
			}
			if report.ID == "" {
				report.ID = reportID
			} else if report.ID != reportID {
				err = errors.Errorf(errors.CodeForbidden, "Cannot change report ID %s to %s", reportID, report.ID)
				util.ErrorHandler(err, w)
				return
			}
			// This call will verify the user actually owns the report
			_, err = tx.GetUserReport(si.UserID, report.ID)
			if util.TXErrorHandler(err, tx, w) != nil {
				return
			}
			// statis & staus_detail cannot be changed from API
			report.Status = ""
			report.StatusDetail = ""
			err = tx.UpdateUserReport(&report)
			if util.TXErrorHandler(err, tx, w) != nil {
				return
			}
			updatedReport, err := tx.GetUserReport(si.UserID, reportID)
			if util.TXErrorHandler(err, tx, w) != nil {
				return
			}
			repCtx := sc.CostDB.NewCostReportContext(updatedReport.ID)
			reprocess, err := repCtx.UpdateRetentionPolicy(updatedReport.RetentionDays)
			if util.TXErrorHandler(err, tx, w) != nil {
				return
			}
			err = tx.Commit()
			if util.TXErrorHandler(err, tx, w) != nil {
				return
			}
			hideAWSSecretAccessKeys(updatedReport)
			util.SuccessHandler(updatedReport, w)
			if reprocess {
				// Notify ingest (in case retention was increased, to reprocess data)
				go sc.NotifyUpdate()
			}
		case "DELETE":
			tx, err := sc.UserDB.Begin()
			if util.TXErrorHandler(err, tx, w) != nil {
				return
			}
			err = tx.DeleteUserReport(si.UserID, reportID)
			if util.TXErrorHandler(err, tx, w) != nil {
				return
			}
			err = tx.Commit()
			if util.TXErrorHandler(err, tx, w) != nil {
				return
			}
			util.SuccessHandler(nil, w)
			go sc.NotifyUpdate()
		default:
			util.ErrorHandler(errors.Errorf(errors.CodeBadRequest, "Unsupported method %s", r.Method), w)
		}
	})
}

// reportStatusHandler is the handler for /v1/reports/{reportID}/status
func reportStatusHandler(sc *server.ServerContext) func(http.ResponseWriter, *http.Request) {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		si, err := sc.SessionManager.ValidateSession(w, r)
		if err != nil {
			return
		}
		vars := mux.Vars(r)
		reportID := vars["reportID"]
		statuses, err := sc.GetUserReportStatus(si.UserID, reportID)
		if util.ErrorHandler(err, w) != nil {
			return
		}
		util.SuccessHandler(statuses, w)
	})
}

// reportAccountsHandler is the handler for /v1/reports/{reportID}/accounts
func reportAccountsHandler(sc *server.ServerContext) func(http.ResponseWriter, *http.Request) {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		si, err := sc.SessionManager.ValidateSession(w, r)
		if err != nil {
			return
		}
		vars := mux.Vars(r)
		reportID := vars["reportID"]
		tx, err := sc.UserDB.Begin()
		if util.TXErrorHandler(err, tx, w) != nil {
			return
		}
		report, err := tx.GetUserReport(si.UserID, reportID)
		if util.TXErrorHandler(err, tx, w) != nil {
			return
		}
		tx.Commit()
		util.SuccessHandler(report.Accounts, w)
	})
}

// reportAccountHandler is the handler for /v1/reports/{reportID}/accounts/{accountID}
func reportAccountHandler(sc *server.ServerContext) func(http.ResponseWriter, *http.Request) {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		si, err := sc.SessionManager.ValidateSession(w, r)
		if err != nil {
			return
		}
		vars := mux.Vars(r)
		reportID := vars["reportID"]
		accountID := vars["accountID"]
		switch r.Method {
		case "GET":
			tx, err := sc.UserDB.Begin()
			if util.ErrorHandler(err, w) != nil {
				return
			}
			report, err := tx.GetUserReport(si.UserID, reportID)
			if util.TXErrorHandler(err, tx, w) != nil {
				return
			}
			tx.Commit()
			for _, a := range report.Accounts {
				if a.AWSAccountID == accountID {
					util.SuccessHandler(a, w)
					return
				}
			}
			err = errors.Errorf(errors.CodeNotFound, "Account %s not found", accountID)
			util.ErrorHandler(err, w)
			return
		case "PUT":
			decoder := json.NewDecoder(r.Body)
			account := userdb.AWSAccountInfo{}
			err = decoder.Decode(&account)
			if err != nil {
				err = errors.New(errors.CodeBadRequest, "Invalid account JSON")
			}
			if util.ErrorHandler(err, w) != nil {
				return
			}
			if account.AWSAccountID == "" {
				account.AWSAccountID = accountID
			} else if account.AWSAccountID != accountID {
				err = errors.Errorf(errors.CodeForbidden, "Cannot change account ID %s to %s", accountID, account.AWSAccountID)
				util.ErrorHandler(err, w)
				return
			}
			tx, err := sc.UserDB.Begin()
			if util.ErrorHandler(err, w) != nil {
				return
			}
			account.ReportID = reportID
			updatedAccount, err := tx.UpdateReportAcccount(&account)
			if util.TXErrorHandler(err, tx, w) != nil {
				return
			}
			tx.Commit()
			util.SuccessHandler(updatedAccount, w)
		default:
			util.ErrorHandler(errors.Errorf(errors.CodeBadRequest, "Unsupported method %s", r.Method), w)
		}
	})
}

// reportBucketsHandler is the handler for /v1/reports/{reportID}/buckets
func reportBucketsHandler(sc *server.ServerContext) func(http.ResponseWriter, *http.Request) {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		si, err := sc.SessionManager.ValidateSession(w, r)
		if err != nil {
			return
		}
		vars := mux.Vars(r)
		reportID := vars["reportID"]

		tx, err := sc.UserDB.Begin()
		if util.TXErrorHandler(err, tx, w) != nil {
			return
		}
		report, err := tx.GetUserReport(si.UserID, reportID)
		if util.TXErrorHandler(err, tx, w) != nil {
			return
		}
		switch r.Method {
		case "GET":
			tx.Commit()
			hideAWSSecretAccessKeys(report)
			util.SuccessHandler(report.Buckets, w)
		case "POST":
			decoder := json.NewDecoder(r.Body)
			bucketCreate := userdb.Bucket{}
			err = decoder.Decode(&bucketCreate)
			if err != nil {
				util.ErrorHandler(errors.New(errors.CodeBadRequest, "Invalid bucket JSON"), w)
				return
			}
			bucketCreate.ReportPath = strings.TrimSpace(bucketCreate.ReportPath)
			bucketCreate.AWSAccessKeyID = strings.TrimSpace(bucketCreate.AWSAccessKeyID)
			bucketCreate.AWSSecretAccessKey = strings.TrimSpace(bucketCreate.AWSSecretAccessKey)
			err = validateBucket(&bucketCreate)
			if util.ErrorHandler(err, w) != nil {
				return
			}
			bucketCreate.ID = ""
			bucketID, err := tx.AddBucket(report.ID, &bucketCreate)
			if util.TXErrorHandler(err, tx, w) != nil {
				return
			}
			createdBucket, err := tx.GetBucket(bucketID)
			if util.TXErrorHandler(err, tx, w) != nil {
				return
			}
			tx.Commit()
			createdBucket.AWSSecretAccessKey = ""
			util.SuccessHandler(createdBucket, w)
			go sc.NotifyUpdate()
		}
	})
}

// reportBucketHandler is the handler for /v1/reports/{reportID}/buckets/{bucketID}
func reportBucketHandler(sc *server.ServerContext) func(http.ResponseWriter, *http.Request) {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		si, err := sc.SessionManager.ValidateSession(w, r)
		if err != nil {
			return
		}
		vars := mux.Vars(r)
		reportID := vars["reportID"]
		bucketID := vars["bucketID"]
		tx, err := sc.UserDB.Begin()
		if util.ErrorHandler(err, w) != nil {
			return
		}
		report, err := tx.GetUserReport(si.UserID, reportID)
		if util.TXErrorHandler(err, tx, w) != nil {
			return
		}
		var bucket *userdb.Bucket
		for _, b := range report.Buckets {
			if b.ID == bucketID {
				bucket = b
				break
			}
		}
		if bucket == nil {
			err = errors.Errorf(errors.CodeNotFound, "Bucket %s not found", bucketID)
			util.TXErrorHandler(err, tx, w)
			return
		}
		switch r.Method {
		case "GET":
			tx.Commit()
			bucket.AWSSecretAccessKey = ""
			util.SuccessHandler(bucket, w)
			return
		case "PUT":
			decoder := json.NewDecoder(r.Body)
			bucketUpdates := userdb.Bucket{}
			err = decoder.Decode(&bucketUpdates)
			if err != nil {
				err = errors.New(errors.CodeBadRequest, "Invalid bucket JSON")
			}
			if util.TXErrorHandler(err, tx, w) != nil {
				return
			}
			if bucketUpdates.ID == "" {
				bucketUpdates.ID = bucketID
			} else if bucketUpdates.ID != bucketID {
				err = errors.Errorf(errors.CodeForbidden, "Cannot change bucket ID %s to %s", bucketID, bucketUpdates.ID)
				util.TXErrorHandler(err, tx, w)
				return
			}
			if bucketUpdates.Bucketname != "" && bucketUpdates.Bucketname != bucket.Bucketname {
				err = errors.Errorf(errors.CodeForbidden, "Bucketnames cannot be changed")
				util.TXErrorHandler(err, tx, w)
				return
			}
			if bucketUpdates.ReportPath != "" && bucketUpdates.ReportPath != bucket.ReportPath {
				err = errors.Errorf(errors.CodeForbidden, "Report paths cannot be changed")
				util.TXErrorHandler(err, tx, w)
				return
			}
			err = validateBucket(&bucketUpdates)
			if util.TXErrorHandler(err, tx, w) != nil {
				return
			}
			err = tx.UpdateBucket(bucketUpdates)
			if util.TXErrorHandler(err, tx, w) != nil {
				return
			}
			updatedBucket, err := tx.GetBucket(bucketID)
			if util.TXErrorHandler(err, tx, w) != nil {
				return
			}
			tx.Commit()
			updatedBucket.AWSSecretAccessKey = ""
			util.SuccessHandler(updatedBucket, w)
			go sc.NotifyUpdate()
		case "DELETE":
			err := tx.DeleteBucket(*bucket)
			if util.TXErrorHandler(err, tx, w) != nil {
				return
			}
			tx.Commit()
			util.SuccessHandler(nil, w)
			go sc.NotifyUpdate()
		default:
			util.TXErrorHandler(errors.Errorf(errors.CodeBadRequest, "Unsupported method %s", r.Method), tx, w)
		}

	})
}

// hideAWSSecretAccessKeys will zero value the aws secret access keys of buckets so that they are not returned by the API
func hideAWSSecretAccessKeys(report *userdb.Report) {
	for _, bucket := range report.Buckets {
		bucket.AWSSecretAccessKey = ""
	}
}
