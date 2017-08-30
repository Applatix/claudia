// Copyright 2017 Applatix, Inc.
package costdb

import (
	"fmt"
	"strings"

	"github.com/applatix/claudia"
	"github.com/applatix/claudia/parser"
)

// Dimension represents a way of grouping or filtering a cost query. Dimensions are nested in a tree structure
type Dimension struct {
	DisplayName string       `json:"display_name"`
	Name        string       `json:"name"`
	UsageUnits  []*UsageUnit `json:"usage_units,omitempty"`
	Dimensions  []*Dimension `json:"dimensions,omitempty"`
}

// UsageUnit is a datastructure representing a usage pricing unit (e.g. GB, GB-Mo, Requests, Hr), and the usage family it applies to
type UsageUnit struct {
	Name          string   `json:"name"`
	UsageFamilies []string `json:"usagefamilies,omitempty"`
}

// GetDimension gets the possible values of a dimension
func (ctx *CostReportContext) GetDimension(dimension string, filters map[string][]string) (*Dimension, error) {
	switch dimension {
	case parser.ColumnService.APIName:
		return ctx.GetServiceDimension(filters)
	case "resourcetags":
		return ctx.GetResourceTagDimension(filters)
	}
	column := parser.APINameToColumn(dimension)
	if column == nil {
		return nil, fmt.Errorf("Dimension %s does not exist", dimension)
	}
	return ctx.getDimension(*column, filters)
}

// getDimension is a helper function to return single dimension given a column and filters
func (ctx *CostReportContext) getDimension(column parser.Column, filters map[string][]string) (*Dimension, error) {
	tagValues, err := ctx.TagValues(column, filters)
	if err != nil {
		return nil, err
	}
	dimValues := make([]*Dimension, len(tagValues))
	for i, tagVal := range tagValues {
		dimVal := Dimension{tagVal, tagVal, nil, nil}
		dimValues[i] = &dimVal
	}
	dimension := Dimension{column.DisplayName, column.APIName, nil, dimValues}
	return &dimension, nil
}

// GetServiceDimension returns a slice of services with their dimensions
func (ctx *CostReportContext) GetServiceDimension(filters map[string][]string) (*Dimension, error) {
	serviceNames, err := ctx.TagValues(parser.ColumnService, filters)
	if err != nil {
		return nil, err
	}
	services := make([]*Dimension, len(serviceNames))
	for i, svcName := range serviceNames {
		dimensions := make([]*Dimension, 0)
		if filters == nil {
			filters = make(map[string][]string)
		}
		// TODO: do not mutate filters
		filters[parser.ColumnService.ColumnName] = []string{svcName}
		if svcName == claudia.ServiceAWSEC2Instance {
			for _, column := range []parser.Column{parser.ColumnEC2InstanceFamily, parser.ColumnEC2InstanceType, parser.ColumnEC2InstancePricing} {
				dimension, err := ctx.getDimension(column, filters)
				if err != nil {
					return nil, err
				}
				dimensions = append(dimensions, dimension)
			}
		} else if svcName == claudia.ServiceAWSS3 {
			for _, column := range []parser.Column{parser.ColumnS3Bucket, parser.ColumnProductFamily, parser.ColumnUsageFamily} {
				dimension, err := ctx.getDimension(column, filters)
				if err != nil {
					return nil, err
				}
				dimensions = append(dimensions, dimension)
			}
		} else if svcName == claudia.ServiceAWSMarketplace {
			dimension, err := ctx.getDimension(parser.ColumnProductCode, filters)
			if err != nil {
				return nil, err
			}
			dimensions = append(dimensions, dimension)
		} else {
			dimension, err := ctx.getDimension(parser.ColumnUsageFamily, filters)
			if err != nil {
				return nil, err
			}
			dimensions = append(dimensions, dimension)
		}

		if hasDataTransfer(dimensions) {
			for _, column := range []parser.Column{parser.ColumnDataTransferSource, parser.ColumnDataTransferDest} {
				dimension, err := ctx.getDimension(column, filters)
				if err != nil {
					return nil, err
				}
				dimensions = append(dimensions, dimension)
			}
		}

		operations, err := ctx.getDimension(parser.ColumnOperation, filters)
		if err != nil {
			return nil, err
		}
		dimensions = append(dimensions, operations)
		usageUnits, err := ctx.GetUsageUnits(svcName)
		if err != nil {
			return nil, err
		}
		services[i] = &Dimension{svcName, svcName, usageUnits, dimensions}
	}
	return &Dimension{"Services", "services", nil, services}, nil
}

// hasDataTransfer checks if any of the dimensions has a usage family type of data transfer
func hasDataTransfer(dimensions []*Dimension) bool {
	// Check if the service has data transfer usage type.
	// If so then source/dest columns should be added as dimensions
	for _, dim := range dimensions {
		if dim.Name == parser.ColumnUsageFamily.APIName {
			for _, subdim := range dim.Dimensions {
				if parser.DataTransferFamilyMatcher.Match([]byte(subdim.Name)) {
					return true
				}
			}
			return false
		}
	}
	return false
}

// GetUsageUnits returns all usage units of a particular service, and the usage families it applies to
func (ctx *CostReportContext) GetUsageUnits(serviceName string) ([]*UsageUnit, error) {
	filters := make(map[string][]string)
	filters[parser.ColumnService.ColumnName] = []string{serviceName}
	unitNames, err := ctx.TagValues(parser.ColumnPricingUnit, filters)
	if err != nil {
		return nil, err
	}
	pricingUnits := make([]*UsageUnit, len(unitNames))
	if len(unitNames) == 1 {
		pricingUnits[0] = &UsageUnit{Name: unitNames[0]}
	} else {
		// If a service has multiple usage units (e.g. S3 has Requests, GB, GB-Mo), then the usage units are specific
		// to classes of usage families. This will perform an additional query to see what usage family the unit applies to
		for i, unitName := range unitNames {
			filters[parser.ColumnPricingUnit.ColumnName] = []string{unitName}
			usageFamilies, err := ctx.TagValues(parser.ColumnUsageFamily, filters)
			if err != nil {
				return nil, err
			}
			pu := UsageUnit{Name: unitName, UsageFamilies: usageFamilies}
			pricingUnits[i] = &pu
		}
	}
	return pricingUnits, nil
}

// Returns all user defined resource tags column names obtained from the cost & usage report (e.g. resourceTag/user:MyTagName)
func (ctx *CostReportContext) getResourceTags() ([]string, error) {
	results, err := ctx.CostDB.Query(fmt.Sprintf("SHOW TAG KEYS FROM %s", ctx.fqMeasurementName))
	if err != nil {
		return nil, err
	}
	resourceTags := make([]string, 0)
	if len(results[0].Series) == 0 {
		return resourceTags, nil
	}
	for _, val := range results[0].Series[0].Values {
		columnName := val[0].(string)
		if parser.ResourceTagMatcher.Match([]byte(columnName)) {
			resourceTags = append(resourceTags, columnName)
		}
	}
	return resourceTags, nil
}

// GetResourceTagDimension returns all resource tags and their possible values
func (ctx *CostReportContext) GetResourceTagDimension(filters map[string][]string) (*Dimension, error) {
	resourceTags, err := ctx.getResourceTags()
	if err != nil {
		return nil, err
	}
	tagDimensions := make([]*Dimension, len(resourceTags))
	for i, columnName := range resourceTags {
		tagName := strings.SplitN(columnName, "/", 2)[1]
		name := fmt.Sprintf("tag:%s", tagName)
		displayName := strings.SplitN(tagName, ":", 2)[1]
		col := parser.Column{ColumnName: columnName, APIName: name, DisplayName: displayName, Parser: nil}
		tagDimension, err := ctx.getDimension(col, filters)
		if err != nil {
			return nil, err
		}
		tagDimensions[i] = tagDimension
	}
	rtDimension := Dimension{"Resource Tags", "resourcetags", nil, tagDimensions}
	return &rtDimension, nil
}
