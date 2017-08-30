// Copyright 2017 Applatix, Inc.
package parser

import (
	"regexp"
	"strings"
)

// AWSRegion contains the name, code (as it appears in billing reports) and display name of an AWS region (e.g. us-west-1, USW1, US West (N. California))
// Some regions have an alternative display name that appears in the description. This is contained in the 'alias' field.
type AWSRegion struct {
	Name        string `json:"name"`
	Code        string `json:"code"`
	DisplayName string `json:"display_name"`
	alias       string
}

// AWS Regions
// The most reliable list of region names (e.g. us-west-1) and full names (e.g. US West (N. California)) is from the SDK:
// * https://github.com/boto/botocore/blob/master/botocore/data/endpoints.json
// The region codes used in the "lineItem/UsageType" column of billing reports (e.g. USW1) are undocumented, but have
// been confirmed via inspection of Billing CSV reports.
// NOTE: the product/location column in the reports will have the display name, e.g.: South America (Sao Paulo)
// Since the billing csv reports do not contain unicode characters, need to sure the display names here do not have accents.
// Otherwise, we will fail to perform an exact string match of product/location to the correct AWSRegion.
var awsRegions = []AWSRegion{
	AWSRegion{"us-east-1", "USE1", "US East (N. Virginia)", "US East (Virginia)"},
	AWSRegion{"us-east-2", "USE2", "US East (Ohio)", ""},
	AWSRegion{"us-west-1", "USW1", "US West (N. California)", "US West (Northern California)"},
	AWSRegion{"us-west-2", "USW2", "US West (Oregon)", ""},
	AWSRegion{"us-gov-west-1", "UGW1", "AWS GovCloud (US)", ""},
	AWSRegion{"ap-northeast-1", "APN1", "Asia Pacific (Tokyo)", ""},
	AWSRegion{"ap-northeast-2", "APN2", "Asia Pacific (Seoul)", ""},
	AWSRegion{"ap-southeast-1", "APS1", "Asia Pacific (Singapore)", ""},
	AWSRegion{"ap-southeast-2", "APS2", "Asia Pacific (Sydney)", ""},
	AWSRegion{"ap-south-1", "APS3", "Asia Pacific (Mumbai)", ""},
	AWSRegion{"eu-central-1", "EUC1", "EU (Frankfurt)", ""},
	AWSRegion{"eu-west-1", "EU", "EU (Ireland)", ""},
	AWSRegion{"eu-west-1", "EUW1", "EU (Ireland)", ""}, // eu-west-1 shows up as both EUW1 and EU in UsageType
	AWSRegion{"eu-west-2", "EUW2", "EU (London)", ""},
	AWSRegion{"sa-east-1", "SAE1", "South America (Sao Paulo)", ""},
	AWSRegion{"ca-central-1", "CAN1", "Canada (Central)", ""},
	//AWSRegion{"cn-north-1", "", "China (Beijing)", ""}, // TODO: Unknown region code
	// Upcoming regions: Ningxia, Paris, Sweden
}

// Generic, catch-all region in which regionless service charges (e.g. Route 53) can be applied
var globalRegion = AWSRegion{"global", "", "Global", ""}

// RegionMapping is a mapping of region names to AWSRegion (e.g. us-west-2 ->  AWSRegion{"us-west-2", "USW2", "US West (Oregon)"})
var RegionMapping map[string]AWSRegion

// regionCodeMapping is a mapping of region codes to AWSRegion (e.g. USW2 ->  AWSRegion{"us-west-2", "USW2", "US West (Oregon)"})
var regionCodeMapping map[string]AWSRegion

// regionDisplayNameMapping is a mapping of region display names to AWSRegion (e.g. US East (N. Virginia) ->  AWSRegion{"us-east-1", "USE1", "US East (N. Virginia)"})
var regionDisplayNameMapping map[string]AWSRegion

// regionDisplayNameMatcher is a regexp to match a region display name: "(US West \(Oregon\)|US East \(N\. Virginia\)|...)"
var regionDisplayNameMatcher *regexp.Regexp

// regionCodeInfixMatcher is a regexp to handle when a region appears an an infix. Regex is built on init, and looks like: "-(USE1|USE2|USW1|USW2|...|...|CAN1)-"
var regionCodeInfixMatcher *regexp.Regexp

var regionMatcher = regexp.MustCompile("^([[:alpha:]]{2}(?:-gov)?-(?:north|northwest|west|southwest|south|southeast|east|northeast|central)-\\d+)-")
var regionCodeMatcher = regexp.MustCompile("^[[:alpha:]]{3}\\d$")

// parseRegionCode parses a 2 or 4 character region code (e.g. USW1) and returns the associated AWSRegion.
// If the code "looks like" a region code, but is not known, this will return an AWSRegion with the
// name, code, and display name all set the same. This allows us to handle new, unanticipated regions
// a bit better. In the UI, it will result in awkward display names, but at least it will not be
// bucketized under "Misc. Charges", nor affect the cardinality of the UsageFamily column.
func parseRegionCode(code string) *AWSRegion {
	region, isKnown := regionCodeMapping[code]
	if isKnown {
		return &region
	}
	if regionCodeMatcher.Match([]byte(code)) {
		// We see a 4 character code which "looks like" a region but we do not know about it.
		// NOTE: making the assumption that this is a region, has a slight risk of amazon coming
		// out with a product code which looks like a region code. For example, hypothetically, a
		// future UsageType could look like "FOO1-ProdUsage" and we would confuse FOO1 as a region.
		return &AWSRegion{Name: code, Code: code, DisplayName: code}
	}
	return nil
}

// parseRegionFailsafe is a fall back mechanism to determine the region from other billing report columns.
// In order of preference: lineItem/AvailabilityZone, product/location, and lineItem/LineItemDescription.
// This is called in the event it was unable to be ascertained from usageType. If region is still unable to
// be determined after examining the three columns, we set the region to the "Global" region.
func parseRegionFailsafe(lineItem LineItem, meta map[string]string) {
	// Determine region based on lineItem/AvailabilityZone
	if aZone, _ := meta[ColumnAvailabilityZone.ColumnName]; aZone != "" {
		// chop off the availability zone letter at the end
		lineItem.Tags[ColumnRegion.ColumnName] = strings.TrimRight(strings.ToLower(aZone), "abcdefghijklmnopqrstuvwxyz")
		return
	}
	// Determine region from product/location
	if location, _ := meta[ColumnProductLocation.ColumnName]; location != "" {
		if region, ok := regionDisplayNameMapping[location]; ok {
			lineItem.Tags[ColumnRegion.ColumnName] = region.Name
			return
		}
		if strings.ToLower(location) == "any" {
			// CodeCommit seems to stuff the word "Any" for regionless. Map it to the global region instead
			lineItem.Tags[ColumnRegion.ColumnName] = globalRegion.Name
			return
		}
		// Location was not empty, but we have never seen it before
		lineItem.Tags[ColumnRegion.ColumnName] = location
		return
	}
	// Determine region from lineItem/LineItemDescription
	// "m4.large Linux/UNIX Spot Instance-hour in US East (Virginia) in VPC Zone #1"
	if description, _ := meta[ColumnDescription.ColumnName]; description != "" {
		regionDisplayName := regionDisplayNameMatcher.FindString(description)
		if regionDisplayName != "" {
			if region, ok := regionDisplayNameMapping[regionDisplayName]; ok {
				lineItem.Tags[ColumnRegion.ColumnName] = region.Name
				return
			}
		}
	}
	// If we get here, it is likely a service that does not have a region (e.g. Route 53)
	// Set to the "global" region.
	lineItem.Tags[ColumnRegion.ColumnName] = globalRegion.Name
}
