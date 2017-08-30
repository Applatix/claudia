// Copyright 2017 Applatix, Inc.
package billingbucket

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path"
	"regexp"
	"strings"

	"github.com/applatix/claudia/errors"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
)

var (
	manifestMatcher = regexp.MustCompile("\\d{8}-\\d{8}/[^/]+-Manifest.json$")
)

// AWSBillingBucket is the object representation of a S3 bucket containing AWS Cost & Usage reports
type AWSBillingBucket struct {
	Bucket     string
	Region     string
	ReportPath string
	Session    *session.Session
	S3Client   *s3.S3

	downloader *s3manager.Downloader
}

// Manifest is the object representation of an AWS Cost & Usage Report Manifest JSON
// Manifest contents look like
//{
//  "assemblyId":"aa1ddccb-abcd-1234-b849-57a32b6864a9",
//  "account":"012345678910",
//  "columns":[{
//    "category":"identity",
//    "name":"LineItemId"
//  },{	// ...
// ...
//  },{
//    "category":"reservation",
//    "name":"TotalReservedUnits"
//  },{
//    "category":"reservation",
//    "name":"UnitsPerReservation"
//  }],
//  "charset":"UTF-8",
//  "compression":"GZIP",
//  "contentType":"text/csv",
//  "reportId":"a5e3b9e6e25825d3591a5bcb9d662104cb4a76a636eb027459266ab31a06d1c3",
//  "reportName":"hourly2",
//  "billingPeriod":{
//    "start":"20161101T000000.000Z",
//    "end":"20161201T000000.000Z"
//  },
//  "bucket":"billing-bucket",
//  "reportKeys":[
//    "report/path/20161101-20161201/aa1ddccb-abcd-1234-b849-57a32b6864a9/hourly2-1.csv.gz",
//    "report/path/20161101-20161201/aa1ddccb-abcd-1234-b849-57a32b6864a9/hourly2-2.csv.gz"
//  ],
//  "additionalArtifactKeys":[{
//    "artifactType":"RedshiftCommands",
//    "name":"report/path/20161101-20161201/aa1ddccb-abcd-1234-b849-57a32b6864a9/hourly2-RedshiftCommands.sql"
//  },{
//    "artifactType":"RedshiftManifest",
//    "name":"report/path/20161101-20161201/aa1ddccb-abcd-1234-b849-57a32b6864a9/hourly2-RedshiftManifest.json"
//  }]
//}
type Manifest struct {
	AssemblyID             string              `json:"assemblyId,omitempty"`
	Account                string              `json:"account,omitempty"`
	Bucket                 string              `json:"bucket,omitempty"`
	BillingPeriod          map[string]string   `json:"billingPeriod,omitempty"`
	Charset                string              `json:"charset,omitempty"`
	Columns                []map[string]string `json:"columns,omitempty"`
	Compression            string              `json:"compression,omitempty"`
	ContentType            string              `json:"contentType,omitempty"`
	ReportID               string              `json:"reportId,omitempty"`
	ReportName             string              `json:"reportName,omitempty"`
	ReportKeys             []string            `json:"reportKeys,omitempty"`
	AdditionalArtifactKeys []interface{}       `json:"additionalArtifactKeys,omitempty"`
}

// BillingPeriodString returns a string representing the billing period (e.g. 20161201-20170101)
func (mfst *Manifest) BillingPeriodString() string {
	return fmt.Sprintf("%s-%s", strings.SplitN(mfst.BillingPeriod["start"], "T", 2)[0], strings.SplitN(mfst.BillingPeriod["end"], "T", 2)[0])
}

// ReportPath returns the reportPath from the reportKey (e.g. "report/path")
func (mfst *Manifest) ReportPath() string {
	for _, reportKey := range mfst.ReportKeys {
		parts := strings.SplitN(reportKey, "/", 3)
		return strings.Join(parts[0:2], "/")
	}
	return ""
}

// categorizeAWSError will return a new error categorized either as caller error or internal error
func categorizeAWSError(err error, defaultMessage string) error {
	log.Printf("Categorizing AWS Error: %s", err.Error())
	if awsErr, ok := err.(awserr.Error); ok {
		switch awsErr.Code() {
		case "NoSuchBucket":
			return errors.Errorf(errors.CodeBadRequest, "Bucket does not exist")
		case "SignatureDoesNotMatch":
			// If credentials are invalid, error will be as follows:
			// [ERROR] SignatureDoesNotMatch: The request signature we calculated does not match the signature you provided. Check your key and signing method.
			return errors.New(errors.CodeBadRequest, "Invalid bucket credentials")
		case "InvalidAccessKeyId":
			return errors.New(errors.CodeBadRequest, "Invalid access key ID")
		case "AccessDenied":
			return errors.New(errors.CodeBadRequest, "Access denied. Check bucket and IAM policy configuration")
		case "UnauthorizedOperation":
			return errors.New(errors.CodeBadRequest, "Unauthorized operation. Check bucket and IAM policy configuration")
		}
	}
	return errors.InternalErrorWithMessage(err, defaultMessage)
}

// GetBucketRegion determines the bucket region using a HTTP HEAD request against a formulated S3 URL.
// We need to correctly match the client region to the bucket's region, otherwise bucket operations fail with:
// BucketRegionError: incorrect region, the bucket is not in 'us-east-1' region.
// A previous implementation used the GetBucketLocation API, but this would intermittently hit AWS error:
// "Insufficient permissions. Check policy configuration"
// Based on the discussions in https://github.com/aws/aws-sdk-go/issues/720, the best way to determine a
// bucket's region is an anonymous HEAD request against the bucket URL and read the "x-amz-bucket-region"
// from the HTTP response headers. Note, however, that anonymous HTTP requests to AWS are subject to much
// stricter AWS API rate limits, and can hit "SlowDown - Please reduce your request rate" errors.
func GetBucketRegion(bucket string) (string, error) {
	url := fmt.Sprintf("https://%s.s3.amazonaws.com", bucket)
	resp, err := http.Head(url)
	if err != nil {
		return "", errors.InternalErrorWithMessage(err, fmt.Sprintf("Failed to determine bucket region: %s", err))
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return "", errors.Errorf(errors.CodeBadRequest, "Bucket '%s' does not exist", bucket)
	}
	for headerKey, values := range resp.Header {
		if strings.ToLower(headerKey) != "x-amz-bucket-region" {
			continue
		}
		for _, region := range values {
			if region != "" {
				return region, nil
			}
		}
	}
	errMsg := fmt.Sprintf("Failed to determine bucket region from '%s' response headers", url)
	log.Printf("%s: response: %v, headers: %s", errMsg, resp, resp.Header)
	return "", errors.New(errors.CodeInternal, errMsg)
}

// NewAWSBillingBucket returns an AWSBillingBucket
func NewAWSBillingBucket(awsAccessKeyID, awsSecretAccessKey, bucket, region, reportPath string) (*AWSBillingBucket, error) {
	billbuck := AWSBillingBucket{}
	billbuck.Bucket = bucket
	billbuck.Region = region
	billbuck.ReportPath = reportPath
	log.Printf("Bucket '%s' located in region: %s", bucket, region)

	cfg := aws.NewConfig()
	if awsAccessKeyID != "" && awsSecretAccessKey != "" {
		creds := credentials.NewStaticCredentials(awsAccessKeyID, awsSecretAccessKey, "")
		cfg.WithCredentials(creds)
	}
	cfg.WithRegion(region)
	// RestProtocolURICleaning needs to be disabled because the SDK will call path.Clean()
	// against S3 object keys, which is not what we want if the key starts with '/'.
	// See: https://github.com/aws/aws-sdk-go/issues/970
	// Object keys beginning with '/' can happen when the user omits setting a "report path prefix"
	// when configuring their billing report.
	cfg.DisableRestProtocolURICleaning = aws.Bool(true)

	// Backdoor to enable AWS logging during requests
	if debug, ok := os.LookupEnv("AWS_DEBUG"); ok && debug == "true" {
		cfg.WithLogLevel(aws.LogDebug | aws.LogDebugWithHTTPBody | aws.LogDebugWithSigning)
	}

	sess, err := session.NewSession(cfg)
	if err != nil {
		return nil, errors.InternalErrorWithMessage(err, "failed to create session")
	}
	billbuck.Session = sess
	billbuck.S3Client = s3.New(sess)
	billbuck.downloader = s3manager.NewDownloaderWithClient(billbuck.S3Client)
	return &billbuck, nil
}

// ListDir treats S3 key like a filesystem and lists the contents of a "directory"
func (billbuck *AWSBillingBucket) ListDir(dir string) ([]string, error) {
	keys := make([]string, 0)
	dir = fmt.Sprintf("%s/", strings.TrimRight(dir, "/"))
	log.Printf("Listing %s/%s", billbuck.Bucket, dir)
	err := billbuck.S3Client.ListObjectsPages(&s3.ListObjectsInput{
		Bucket:    &billbuck.Bucket,
		Prefix:    &dir,
		Delimiter: aws.String("/"),
	}, func(p *s3.ListObjectsOutput, last bool) (shouldContinue bool) {
		// Contents contains the "files"
		for _, obj := range p.Contents {
			keys = append(keys, *obj.Key)
		}
		// CommonPrefixes contains the "directories"
		for _, cp := range p.CommonPrefixes {
			keys = append(keys, *cp.Prefix)
		}
		return true
	})
	if err != nil {
		log.Println("Failed to list contents of bucket")
		return nil, categorizeAWSError(err, "failed to list: "+dir)
	}
	return keys, nil
}

// GetManifestPaths returns the paths to all manifests in the bucket (e.g. report/path/20161201-20170101/hourly2-Manifest.json)
func (billbuck *AWSBillingBucket) GetManifestPaths() ([]string, error) {
	manifestDirs, err := billbuck.ListDir(billbuck.ReportPath)
	if err != nil {
		return nil, err
	}
	log.Printf("Found manifest dirs %s", manifestDirs)
	manifestPaths := make([]string, 0)
	for _, manifestDir := range manifestDirs {
		dirContents, err := billbuck.ListDir(manifestDir)
		if err != nil {
			return nil, err
		}
		for _, path := range dirContents {
			if manifestMatcher.MatchString(path) {
				manifestPaths = append(manifestPaths, path)
			}
		}
	}
	return manifestPaths, nil
}

// Download will download the file into given directory or file path, creating directory structure if necessary.
// Optionally skips files which are same size
func (billbuck *AWSBillingBucket) Download(key string, downloadPath string, skipIfSizeIdentical bool) error {
	downloadPath = path.Clean(downloadPath)
	fi, err := os.Stat(downloadPath)
	var existingFileSize int64 = -1
	if err != nil {
		if !os.IsNotExist(err) {
			return errors.InternalError(err)
		}
		// path does not exist
		err = os.MkdirAll(path.Dir(downloadPath), 0700)
		if err != nil {
			return errors.InternalError(err)
		}
	} else {
		if fi.IsDir() {
			downloadPath = fmt.Sprintf("%s/%s", downloadPath, path.Base(key))
			if fi, err := os.Stat(downloadPath); err == nil {
				existingFileSize = fi.Size()
			}
		} else {
			existingFileSize = fi.Size()
		}
	}

	obj, err := billbuck.S3Client.GetObject(&s3.GetObjectInput{
		Bucket: aws.String(billbuck.Bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return errors.InternalError(err)
	}
	defer obj.Body.Close()
	// If local file exists, check size of obj to see if we really need to redownload it
	if existingFileSize >= 0 {
		log.Printf("Report already exists at %s", downloadPath)
		if skipIfSizeIdentical && existingFileSize == *obj.ContentLength {
			log.Printf("File sizes identical (%d). Skipping download", existingFileSize)
			return nil
		}
	}
	log.Printf("Downloading %s/%s to %s", billbuck.Bucket, key, downloadPath)
	file, err := os.Create(downloadPath)
	if err != nil {
		return errors.InternalError(err)
	}
	defer file.Close()

	numBytes, err := billbuck.downloader.Download(file,
		&s3.GetObjectInput{
			Bucket: aws.String(billbuck.Bucket),
			Key:    aws.String(key),
		})
	if err != nil {
		return errors.InternalError(err)
	}
	log.Printf("Download completed (%d bytes)", numBytes)
	return nil
}

// GetManifest returns a manifest object for the manifest at the given key
func (billbuck *AWSBillingBucket) GetManifest(key string) (*Manifest, error) {
	log.Printf("Retrieving manifest %s/%s", billbuck.Bucket, key)
	obj, err := billbuck.S3Client.GetObject(&s3.GetObjectInput{
		Bucket: aws.String(billbuck.Bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return nil, errors.InternalError(err)
	}
	defer obj.Body.Close()
	data, err := ioutil.ReadAll(obj.Body)
	if err != nil {
		return nil, errors.InternalError(err)
	}
	var manifest Manifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return nil, errors.InternalError(err)
	}
	return &manifest, nil
}
