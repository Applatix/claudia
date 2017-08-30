// Copyright 2017 Applatix, Inc.
package claudia

import "time"

// Application constants
const (
	AWSMarketplaceAccountID = "679593333241"
)

// Specially treated services
const (
	ServiceAWSS3              = "AWS S3"
	ServiceAWSEC2Instance     = "AWS EC2 Instance"
	ServiceAWSMarketplace     = "AWS Marketplace"
	ServiceAWSEBSVolume       = "AWS EBS Volume"
	ServiceAWSCloudWatch      = "AWS CloudWatch"
	ServiceAWSCloudFront      = "AWS CloudFront"
	ServiceAWSEC2DataTransfer = "AWS EC2 Data Transfer"
)

// Application configuration settings
var (
	ApplicationPort                 = 443
	ApplicationDir                  = "/var/lib/claudia"
	ApplicationAdminUsername        = "admin"
	ApplicationAdminDefaultPassword = "password"
	IngestdURL                      = "http://ingestd:8081"
	IngestdPort                     = 8081
	IngestdWorkers                  = 2
	IngestdBatchInterval            = 5000
	IngestStatusMeasurementName     = "ingest_status"
	IngestdWriteRetryDelay          = []time.Duration{10 * time.Second, 20 * time.Second, 40 * time.Second, 60 * time.Second, 120 * time.Second}
	CostDatabaseURL                 = "http://costdb:8086"
	CostDatabaseName                = "cost_usage"
	ReportDefaultRetentionDays      = 365
)

// ReportStatus is the status of a report. One of: "processing", "error", "current"
type ReportStatus string

// Valid report statuses
const (
	ReportStatusCurrent    ReportStatus = "current"
	ReportStatusError      ReportStatus = "error"
	ReportStatusProcessing ReportStatus = "processing"
)
