// Copyright 2017 Applatix, Inc.
package parser

import (
	"fmt"
	"log"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/applatix/claudia"
	"github.com/applatix/claudia/errors"
)

// ParserVersion is the version of this parser library, which we record in the event that billing
// data needs to be reingested in later versions of this software. Until we support upgrades,
// this value can remain at 1. When upgrade is supported, we will need to increment this
// value every time we make incompatible changes to the parser.
const ParserVersion = 1

// The Column struct represents:
// * the column name of an AWS Cost & Usage report line item (e.g. lineItem/ProductCode)
// * the database tag/field name (e.g. claudia/Region)
// * the API shorthand API name (e.g. "regions")
// Also, parser how to parse the
type Column struct {
	ColumnName  string
	APIName     string
	DisplayName string
	Parser      ColumnParser
}

// For more details of AWS columns:
// http://docs.aws.amazon.com/awsaccountbilling/latest/aboutv2/detailed-billing-reports.html
var (
	// Fields
	ColumnUnblendedCost = Column{"lineItem/UnblendedCost", "", "", asFloatField} // 1.04  (will be zero for ReservedInstance)
	ColumnBlendedCost   = Column{"lineItem/BlendedCost", "", "", asFloatField}   // 1.04
	ColumnUsageAmount   = Column{"lineItem/UsageAmount", "", "", asFloatField}   // 20
	ColumnUnblendedRate = Column{"lineItem/UnblendedRate", "", "", asFloatField} // 0.052
	ColumnBlendedRate   = Column{"lineItem/BlendedRate", "", "", asFloatField}   // 0.052
	ColumnLineItemID    = Column{"identity/LineItemId", "", "", asStringField}   // qwhyoiu7gg3wow4eskqclrsuzfthtct4pwqnjkismbrsvqkepmxq
	ColumnResourceID    = Column{"lineItem/ResourceId", "", "", asStringField}   // i-abcd1234, vol-abcd1234, my-billing-bucket
	ColumnUsageType     = Column{"lineItem/UsageType", "", "", usageTypeParser}  // USW2-BoxUsage:t1.micro

	// Tags
	ColumnPayerAccountID = Column{"bill/PayerAccountId", "", "", asTag}                                // 012345678910
	ColumnUsageAccountID = Column{"lineItem/UsageAccountId", "accounts", "Accounts", asTag}            // 246810121416
	ColumnProductCode    = Column{"lineItem/ProductCode", "products", "Products", asTag}               // AmazonEC2, a6vjvrelz10rgvvemklxv2dow, awskms, AWSCloudTrail
	ColumnOperation      = Column{"lineItem/Operation", "operations", "Operations", asTag}             // RunInstances, Hourly, GetObject, NatGateway, Send, Unknown
	ColumnProductFamily  = Column{"product/productFamily", "productfamilies", "Product Family", asTag} // * Compute Instance, Storage, Storage Snapshot, NAT Gateway
	ColumnPricingUnit    = Column{"pricing/unit", "", "", asTag}                                       // * Hrs, Queries, Requests, GB, GB-Mo, Events, IOs, Keys, Count, ReadCapacityUnit-Hrs, WriteCapacityUnit-Hrs

	// Meta
	ColumnPricingTerm            = Column{"pricing/term", "", "", asMeta}                 // * OnDemand, Reserved (empty if lineItem/UnblendedCost is 0.0)
	ColumnBillingPeriodStartDate = Column{"bill/BillingPeriodStartDate", "", "", asMeta}  // 2016-12-01T00:00:00Z
	ColumnBillingPeriodEndDate   = Column{"bill/BillingPeriodEndDate", "", "", asMeta}    // 2017-01-01T00:00:00Z
	ColumnAvailabilityZone       = Column{"lineItem/AvailabilityZone", "", "", asMeta}    // us-east-1d
	ColumnProductLocation        = Column{"product/location", "", "", asMeta}             // US East (N. Virginia)
	ColumnDescription            = Column{"lineItem/LineItemDescription", "", "", asMeta} // m4.large Linux/UNIX Spot Instance-hour in US East (Virginia) in VPC Zone #1

	// Claudia specific DB columns
	ColumnBillingPeriod      = Column{"claudia/BillingPeriod", "", "", nil}                                    // 20161201-20170101
	ColumnBillingBucket      = Column{"claudia/BillingBucket", "", "", nil}                                    // my-billing-bucket
	ColumnBillingReportPath  = Column{"claudia/BillingReportPath", "", "", nil}                                // report/path
	ColumnService            = Column{"claudia/Service", "services", "Services", nil}                          // * AWS EC2 Instance
	ColumnS3Bucket           = Column{"claudia/S3Bucket", "s3buckets", "Buckets", nil}                         // * my-billing-bucket
	ColumnUsageFamily        = Column{"claudia/UsageFamily", "usagefamilies", "Usage Family", nil}             // * Requests-Tier1, AWS-Out-Bytes, NatGateway-Bytes
	ColumnRegion             = Column{"claudia/Region", "regions", "Regions", nil}                             // * us-east-1, us-east-2
	ColumnEC2InstancePricing = Column{"claudia/EC2Pricing", "instancepricing", "Instance Pricing", nil}        // * OnDemand, Reserved, Spot
	ColumnEC2InstanceFamily  = Column{"claudia/EC2InstanceFamily", "instancefamilies", "Instance Family", nil} // * m3
	ColumnEC2InstanceType    = Column{"claudia/EC2InstanceType", "instancetypes", "Instance Type", nil}        // * m3.large
	ColumnDataTransferSource = Column{"claudia/DataTransferSource", "txsource", "Data Transfer Source", nil}   // * External, us-west-1
	ColumnDataTransferDest   = Column{"claudia/DataTransferDest", "txdest", "Data Transfer Dest", nil}         // * External, us-west-1
)

// Add all columns to internal array to be used to build up lookup tables during init()
var columns = []Column{
	// Fields
	ColumnUnblendedCost,
	ColumnBlendedCost,
	ColumnUsageAmount,
	ColumnUnblendedRate,
	ColumnBlendedRate,
	ColumnLineItemID,
	ColumnResourceID,
	ColumnUsageType,

	// Tags
	ColumnPayerAccountID,
	ColumnUsageAccountID,
	ColumnProductCode,
	ColumnOperation,
	ColumnProductFamily,
	ColumnPricingUnit,

	// Meta
	ColumnPricingTerm,
	ColumnBillingPeriodStartDate,
	ColumnBillingPeriodEndDate,
	ColumnAvailabilityZone,
	ColumnProductLocation,
	ColumnDescription,

	// Claudia columns
	ColumnBillingPeriod,
	ColumnBillingBucket,
	ColumnBillingReportPath,
	ColumnService,
	ColumnS3Bucket,
	ColumnUsageFamily,
	ColumnRegion,
	ColumnEC2InstancePricing,
	ColumnEC2InstanceFamily,
	ColumnEC2InstanceType,
	ColumnDataTransferSource,
	ColumnDataTransferDest,
}

// Other candidate columns from the CSV file to consider parsing
//"bill/InvoiceId":            // 24681012
//"bill/BillingEntity":        // AWS, AWS Marketplace
//"bill/BillType":             // Anniversary, Purchase
//"lineItem/UsageStartDate":   // 2016-10-01T00:00:00Z
//"lineItem/LineItemType":     // Usage, DiscountedUsage, Credit, Purchase, Fee, RIFee
//"lineItem/AvailabilityZone": // us-west-2a
//"product/ProductName":       // Amazon Elastic Compute Cloud, Amazon Simple Storage Service
//"product/group":             // ELB:Balancer, NGW:NatGateway, S3-API-Tier2, ElasticIP:Address
//"product/location":          // US West (Oregon)
//"product/operation":         // RunInstances, NatGateway, LoadBalancing
//"product/servicecode":       // AmazonEC2, AWSDataTransfer
//"product/usagetype":         // USW2-SAE1-AWS-In-Bytes,
//"product/instanceType":      // m4.large, t2.large, t2.medium, t2.micro
//"reservation/ReservationARN":// arn:aws:ec2:us-west-2:012345678910:reserved-instances/1702ffb5-06cb-48c0-8852-8232a4748fe9

// ColumnParser returns a ParsedValues structure, which consists of fields, tags, and metadata
// Fields are the units by which we want to measure. They can be numbers or strings.
// Tags are string-based column names which are indexed and can be filtered/grouped. The number of Tags should be
// limited in order to limit database cardinality, but sufficient enough to provide desired querying capabilities.
// NOTE: influxdb uses tag sets to prevent duplicate points, but we circumvent this by using nanosecond sequence numbers
// Added to each timestamp.
// https://docs.influxdata.com/influxdb/v1.1/troubleshooting/frequently-asked-questions/#how-does-influxdb-handle-duplicate-points
// Metadata is used as a temporary holding area in which we need to process line items
type ColumnParser func(string, string) (*parsedValues, error)

type parsedValues struct {
	Tags   map[string]string
	Fields map[string]interface{}
	Meta   map[string]string // meta are columns we may not want to store in DB but are used in the line parser
}

// Mapping of csv column name to Column object
var columnMapping map[string]Column

// APIColumnMapping is a mapping of API name to column name (e.g. product -> lineItem/ProductCode)
var apiColumnMapping map[string]Column

// APINameToColumn returns the database column name given an API name
func APINameToColumn(apiname string) *Column {
	column, exists := apiColumnMapping[apiname]
	if exists {
		return &column
	}
	if strings.HasPrefix(apiname, "tag:") {
		columnName := fmt.Sprintf("resourceTags/%s", apiname[4:])
		column = Column{ColumnName: columnName, APIName: apiname, DisplayName: apiname, Parser: nil}
		return &column
	}
	return nil
}

// APINameToColumnName returns the database column name given an API name
func APINameToColumnName(apiname string) *string {
	column := APINameToColumn(apiname)
	if column == nil {
		return nil
	}
	return &column.ColumnName
}

// GetColumnByName returns the database column given an column name
func GetColumnByName(columnName string) *Column {
	column, ok := columnMapping[columnName]
	if !ok {
		return nil
	}
	return &column
}

func init() {
	regionCodeMapping = make(map[string]AWSRegion)
	regionDisplayNameMapping = make(map[string]AWSRegion)
	RegionMapping = make(map[string]AWSRegion)
	regionCodes := make([]string, len(awsRegions))
	regionDisplayNames := make([]string, 0)
	for i, region := range awsRegions {
		regionCodeMapping[region.Code] = region
		regionCodes[i] = region.Code
		if region.Code == "EU" {
			// EUW1 and EU are synonyms. Only need to map EUW1
			continue
		}
		regionDisplayNameMapping[region.DisplayName] = region
		RegionMapping[region.Name] = region
		regionDisplayNames = append(regionDisplayNames, regexp.QuoteMeta(region.DisplayName))
		if region.alias != "" {
			regionDisplayNameMapping[region.alias] = region
			regionDisplayNames = append(regionDisplayNames, regexp.QuoteMeta(region.alias))
		}
	}
	regionCodeInfixMatcher = regexp.MustCompile("-(" + strings.Join(regionCodes, "|") + ")-")
	// regex which can find a region display name (used to search the lineItemDescription)
	regionDisplayNameMatcher = regexp.MustCompile("(" + strings.Join(regionDisplayNames, "|") + ")")
	RegionMapping[globalRegion.Name] = globalRegion

	columnMapping = make(map[string]Column, 0)
	apiColumnMapping = make(map[string]Column, 0)
	for _, column := range columns {
		columnMapping[column.ColumnName] = column
		if column.APIName != "" {
			apiColumnMapping[column.APIName] = column
		}
	}
}

// Other Regexes used during parsing
var (
	ResourceTagMatcher        = regexp.MustCompile("^resourceTags/(user|aws):.*")
	instanceTypeRegexStr      = "([[:alpha:]]\\d+)\\.(?:\\w*)(?:nano|micro|small|medium|large)" // See: https://aws.amazon.com/ec2/instance-types/
	instanceTypeMatcher       = regexp.MustCompile(fmt.Sprintf("^(.*)[:\\.](%s)$", instanceTypeRegexStr))
	errorMatcher              = regexp.MustCompile("^\\[Error:.*\\]$") // To ignore errors appearing in lineItem/ResourceId
	dataTransferFamilyStr     = "(AWS-In-Bytes|AWS-Out-Bytes|DataTransfer-Regional-Bytes|DataTransfer-Out-Bytes|DataTransfer-In-Bytes)"
	DataTransferFamilyMatcher = regexp.MustCompile("^" + dataTransferFamilyStr + "$")
	dataTransferMatcher       = regexp.MustCompile("^(?:(.+)-)?(?:(.+)-)" + dataTransferFamilyStr + "$")
)

// Parsers

func asTag(columnName, columnValue string) (*parsedValues, error) {
	tags := map[string]string{columnName: columnValue}
	dp := parsedValues{Tags: tags, Fields: nil, Meta: nil}
	return &dp, nil
}

func asFloatField(columnName, columnValue string) (*parsedValues, error) {
	floatVal, err := strconv.ParseFloat(columnValue, 0)
	if err != nil {
		return nil, err
	}
	fields := map[string]interface{}{columnName: floatVal}
	dp := parsedValues{Tags: nil, Fields: fields, Meta: nil}
	return &dp, nil
}

func asStringField(columnName, columnValue string) (*parsedValues, error) {
	fields := map[string]interface{}{columnName: columnValue}
	dp := parsedValues{Tags: nil, Fields: fields, Meta: nil}
	return &dp, nil
}

func asMeta(columnName, columnValue string) (*parsedValues, error) {
	meta := map[string]string{columnName: columnValue}
	dp := parsedValues{Tags: nil, Fields: nil, Meta: meta}
	return &dp, nil
}

// usageTypeParser parses out various information from the usage type
// Sets the following fields:
// * lineItem/UsageType
// Sets the following tags:
// * claudia/UsageFamily
// * claudia/Region
// * claudia/EC2InstanceFamily
// * claudia/EC2InstanceType
// * claudia/DataTransferSource
// * claudia/DataTransferDest
func usageTypeParser(columnName, usageType string) (*parsedValues, error) {
	fields := make(map[string]interface{})
	tags := make(map[string]string)

	fields[columnName] = usageType
	var parts []string

	parts = instanceTypeMatcher.FindStringSubmatch(usageType)
	usageFamily := usageType
	if len(parts) > 0 {
		usageFamily = parts[1]
		tags[ColumnEC2InstanceType.ColumnName] = parts[2]
		tags[ColumnEC2InstanceFamily.ColumnName] = parts[3]
	}
	// Determines region (if present). Strips out beginning region codes from being included in UsageFamily
	parts = strings.SplitN(usageFamily, "-", 3)
	var idx int
	for i, part := range parts {
		idx = i
		region := parseRegionCode(part)
		if region == nil {
			break
		} else {
			if i == 0 {
				tags[ColumnRegion.ColumnName] = region.Name
			}
		}
	}
	usageFamily = strings.Join(parts[idx:], "-")

	addDataTransferTags(usageType, tags)

	if _, ok := tags[ColumnRegion.ColumnName]; !ok {
		// If we have yet to set the region, fall back to some more arcane region detection mechanisms.
		// Some region names are contained in beginning of UsageType (e.g. us-east-1-KMS-Requests). Strip those out too
		regionName := regionMatcher.FindString(usageFamily)
		if regionName != "" {
			tags[ColumnRegion.ColumnName] = regionName[0 : len(regionName)-1]
			usageFamily = usageFamily[len(regionName):]
		} else {
			// Some region codes appear as an infix, in the *middle* of UsageType (e.g. WorkDocs-USE1-InclStorageByteHrs, WorkDocs-USE1-WSOnly-UserHrs).
			// *sigh* Strip those out too. UsageFamily will now become: WorkDocs-InclStorageByteHrs
			regionCode := regionCodeInfixMatcher.FindString(usageFamily)
			if regionCode != "" {
				regionCode = regionCode[1 : len(regionCode)-1]
				if region, exists := regionCodeMapping[regionCode]; exists {
					tags[ColumnRegion.ColumnName] = region.Name
					usageFamily = strings.Replace(usageFamily, regionCode+"-", "", 1)
				}
			}
		}
	}

	tags[ColumnUsageFamily.ColumnName] = usageFamily
	return &parsedValues{Tags: tags, Fields: fields, Meta: nil}, nil
}

// addDataTransferTags add source and dest data transfer locations as tags
func addDataTransferTags(usageType string, tags map[string]string) {
	txParts := dataTransferMatcher.FindStringSubmatch(usageType)
	if len(txParts) != 4 {
		return
	}
	var src, dst string
	if txParts[1] != "" {
		// We get here if there are two regions associated with the transfer.
		// (e.g. USW2-SAE1-AWS-In-Bytes)
		if strings.HasSuffix(usageType, "-In-Bytes") {
			src = txParts[2]
			dst = txParts[1]
		} else {
			src = txParts[1]
			dst = txParts[2]
		}
	} else {
		// We get here if there is only one region associated with the transfer.
		// Meaning it is either an intra-region (AKA regional) transfer (e.g. USW2-DataTransfer-Regional-Bytes),
		// or it is an external data transfer outside AWS (e.g. USW2-DataTransfer-In-Bytes)
		if strings.HasSuffix(usageType, "-Regional-Bytes") {
			// Inter-region transfer
			src = txParts[2]
			dst = txParts[2]
		} else {
			// External transfer
			if strings.HasSuffix(usageType, "-In-Bytes") {
				src = "External"
				dst = txParts[2]
			} else {
				src = txParts[2]
				dst = "External"
			}
		}
	}
	if region, isRegion := regionCodeMapping[src]; isRegion {
		src = region.Name
	}
	if region, isRegion := regionCodeMapping[dst]; isRegion {
		dst = region.Name
	}
	tags[ColumnDataTransferSource.ColumnName] = src
	tags[ColumnDataTransferDest.ColumnName] = dst
}

// LineItem is the parsed form of a single line in a AWS Cost & Usage billing report.
// It indicates out what columns should be stored (indexed) as tags vs. fields when stored to the database.
type LineItem struct {
	Timestamp time.Time
	Tags      map[string]string
	Fields    map[string]interface{}
}

// ParseLine parses a CSV line and return a InfluxDB point
func ParseLine(columnNames []string, line []string) (*LineItem, error) {
	var lineItem LineItem
	lineItem.Tags = make(map[string]string)
	lineItem.Fields = make(map[string]interface{})
	meta := make(map[string]string)
	var err error

	for i, value := range line {
		columnName := columnNames[i]
		if columnName == "identity/TimeInterval" {
			parts := strings.Split(value, "/")
			startHr := parts[0]
			endHr := parts[1]
			var startTime, endTime time.Time
			startTime, err = time.Parse(time.RFC3339, startHr)
			if err != nil {
				return nil, errors.InternalError(err)
			}
			endTime, err = time.Parse(time.RFC3339, endHr)
			if err != nil {
				return nil, errors.InternalError(err)
			}
			if endTime.Sub(startTime) != time.Hour {
				// AWS report line items that span more than an hour are aggregated values and should be ignored
				log.Printf("Skipping non hour duration")
				return nil, nil
			}
			lineItem.Timestamp = startTime
			continue
		}
		if columnName == "lineItem/LineItemType" && value != "Usage" && value != "DiscountedUsage" {
			// Ignore non usage line items
			log.Printf("Skipping non usage line")
			return nil, nil
		}
		if value == "" {
			if columnName == ColumnProductFamily.ColumnName {
				// We specially treat lineItem/ProductFamily by setting it to be "Other" to fill in the gaps and so
				// that every line item has a non-empty value for product family
				lineItem.Tags[ColumnProductFamily.ColumnName] = "Other"
			}
			continue
		}
		if ResourceTagMatcher.MatchString(columnName) {
			lineItem.Tags[columnName] = value
			continue
		}
		column, doParse := columnMapping[columnName]
		if !doParse {
			continue
		}
		if columnName == ColumnResourceID.ColumnName && errorMatcher.MatchString(value) {
			// Sometimes errors appear in the lineItem/ResourceId column of billing reports
			// (e.g [Error:OperationAborted]). Do not store these errors as resource ids.
			continue
		}
		parsedVals, err := column.Parser(columnName, value)
		if err != nil {
			return nil, errors.InternalErrorf(err, "Failed to parse column %s (%s): %s", columnName, value, err)
		}
		for k, v := range parsedVals.Tags {
			lineItem.Tags[k] = v
		}
		for k, v := range parsedVals.Fields {
			lineItem.Fields[k] = v
		}
		for k, v := range parsedVals.Meta {
			meta[k] = v
		}
	}
	// Index S3 buckets
	productCode, _ := lineItem.Tags[ColumnProductCode.ColumnName]
	if productCode == "AmazonS3" {
		if resourceID, exists := lineItem.Fields[ColumnResourceID.ColumnName]; exists {
			lineItem.Tags[ColumnS3Bucket.ColumnName] = resourceID.(string)
		}
	}
	// Set pricing term (Spot, OnDemand, Reserved)
	pricingTerm, _ := meta[ColumnPricingTerm.ColumnName]
	if pricingTerm != "" {
		lineItem.Tags[ColumnEC2InstancePricing.ColumnName] = pricingTerm
	} else if lineItem.Tags[ColumnUsageFamily.ColumnName] == "SpotUsage" {
		lineItem.Tags[ColumnEC2InstancePricing.ColumnName] = "Spot"
	} else if lineItem.Tags[ColumnProductCode.ColumnName] == "AmazonEC2" && lineItem.Tags[ColumnProductFamily.ColumnName] == "Compute Instance" {
		lineItem.Tags[ColumnEC2InstancePricing.ColumnName] = "OnDemand"
	}

	// Handles the case where region information was not in the usageType column
	if _, ok := lineItem.Tags[ColumnRegion.ColumnName]; !ok {
		parseRegionFailsafe(lineItem, meta)
	}

	// Opinionated categorizations of products into "Service" column.
	// NOTE: the order here matters
	if lineItem.Tags[ColumnProductCode.ColumnName] == "AmazonEC2" {
		// AmazonEC2 is broken into the following services:
		// * AWS EC2 Instance
		// * AWS EC2 Data Transfer
		// * AWS EC2 IP Address
		// * AWS EC2 Load Balancer
		// * AWS EC2 NAT Gateway
		// * AWS EBS Volume
		// * AWS CloudWatch (combined with AWSCloudWatch product)
		// * AWS CloudFront
		if lineItem.Tags[ColumnProductFamily.ColumnName] == "Compute Instance" {
			lineItem.Tags[ColumnService.ColumnName] = claudia.ServiceAWSEC2Instance
		} else if lineItem.Tags[ColumnUsageFamily.ColumnName] == "SpotUsage" {
			lineItem.Tags[ColumnService.ColumnName] = claudia.ServiceAWSEC2Instance
		} else if strings.HasPrefix(lineItem.Tags[ColumnUsageFamily.ColumnName], "EBS:") {
			lineItem.Tags[ColumnService.ColumnName] = claudia.ServiceAWSEBSVolume
		} else if strings.HasPrefix(lineItem.Tags[ColumnUsageFamily.ColumnName], "CW:") {
			lineItem.Tags[ColumnService.ColumnName] = claudia.ServiceAWSCloudWatch
		} else if lineItem.Tags[ColumnProductFamily.ColumnName] == "Data Transfer" {
			if strings.HasPrefix(lineItem.Tags[ColumnUsageFamily.ColumnName], "CloudFront-") {
				lineItem.Tags[ColumnService.ColumnName] = claudia.ServiceAWSCloudFront
			} else {
				lineItem.Tags[ColumnService.ColumnName] = claudia.ServiceAWSEC2DataTransfer
			}
		} else if lineItem.Tags[ColumnProductFamily.ColumnName] != "" && lineItem.Tags[ColumnProductFamily.ColumnName] != "Other" {
			// We make all the sub categories of AmazonEC2 top level services (e.g. IP Address, Load Balancer, NAT Gateway)
			lineItem.Tags[ColumnService.ColumnName] = fmt.Sprintf("AWS EC2 %s", lineItem.Tags[ColumnProductFamily.ColumnName])
		} else {
			// If we get here, ProductCode == "AmazonEC2" and (ProductFamily == "" || ProductFamily == "Other")
			// This can happen for AWS Marketplace 3rd party services (e.g. OpenVPN), which will have an empty ProductFamily
			// See if we can determine the service name based on the UsageFamily
			if DataTransferFamilyMatcher.Match([]byte(lineItem.Tags[ColumnUsageFamily.ColumnName])) {
				// UsageFamily looks like data transfer
				lineItem.Tags[ColumnService.ColumnName] = claudia.ServiceAWSEC2DataTransfer
			} else if lineItem.Tags[ColumnUsageFamily.ColumnName] == "BoxUsage" {
				// We have seen ProductFamily be blank some instances p2.8xlarge, p2.xlarge in n ap-southeast-1
				lineItem.Tags[ColumnService.ColumnName] = claudia.ServiceAWSEC2Instance
			} else {
				// If we get here, we do not know how to categorize this. Place under an "EC2 Other" category
				// We should never reach this, otherwise the parser should be updated
				lineItem.Tags[ColumnService.ColumnName] = "AWS EC2 Other"
			}
		}
	} else {
		// Makes consistent Amazon and AWS product codes with just "AWS"
		// Combine 3rd party products into a service called "AWS Marketplace"
		service := lineItem.Tags[ColumnProductCode.ColumnName]
		serviceLower := strings.ToLower(service)
		if strings.HasPrefix(serviceLower, "amazon") {
			lineItem.Tags[ColumnService.ColumnName] = "AWS " + strings.Trim(service[6:], " ")
		} else if strings.HasPrefix(serviceLower, "aws") {
			lineItem.Tags[ColumnService.ColumnName] = "AWS " + strings.Trim(service[3:], " ")
		} else {
			// If we get here, the ProductCode does not start with "amazon" or "aws". Assume 3rd party
			lineItem.Tags[ColumnService.ColumnName] = claudia.ServiceAWSMarketplace
		}
	}
	// If pricing/unit is blank, see if we can infer it from UsageFamily and/or other fields
	pricingUnit, _ := lineItem.Tags[ColumnPricingUnit.ColumnName]
	if pricingUnit == "" {
		usageFamily := lineItem.Tags[ColumnUsageFamily.ColumnName]
		if strings.HasSuffix(usageFamily, "-Bytes") {
			lineItem.Tags[ColumnPricingUnit.ColumnName] = "GB"
		} else if strings.HasPrefix(usageFamily, "Requests-") || strings.HasSuffix(usageFamily, "-Requests") {
			lineItem.Tags[ColumnPricingUnit.ColumnName] = "Requests"
		} else if strings.HasSuffix(usageFamily, "-ByteHrs") || strings.HasSuffix(usageFamily, "-Storage") {
			lineItem.Tags[ColumnPricingUnit.ColumnName] = "GB-Mo"
		} else if strings.HasSuffix(usageFamily, "-Hours") || lineItem.Tags[ColumnService.ColumnName] == claudia.ServiceAWSEC2Instance {
			lineItem.Tags[ColumnPricingUnit.ColumnName] = "Hrs"
		} else if strings.HasPrefix(usageFamily, "Recipients") {
			lineItem.Tags[ColumnPricingUnit.ColumnName] = "Count"
		}
	}
	// cost reports can have inconsistent casing for "pricing/unit" (e.g. "Hrs" vs. "hrs")
	if pricingUnit, ok := lineItem.Tags[ColumnPricingUnit.ColumnName]; ok {
		lineItem.Tags[ColumnPricingUnit.ColumnName] = strings.ToLower(pricingUnit)
	}
	return &lineItem, nil
}
