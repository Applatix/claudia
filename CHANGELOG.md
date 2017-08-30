# Changelog

### Version 1.1.0 (2017-08-30)
 * Open source release
 * Cache bucket region during configuration

### Version 1.0.9 (2017-04-27)
 * Fix issue preventing the ingestion of reports with Redshift/QuickSight option enabled
 * Better error reporting to UI

### Version 1.0.8 (2017-04-25)
 * Fix issue where some metrics (e.g. instance hours) would appear twice in the metrics selector

### Version 1.0.7 (2017-04-20)
 * Eliminate the GetBucketLocation requirement for the Claudia IAM policy

### Version 1.0.6 (2017-04-11)
 * Accomodate changes to AWS Cost & Usage reports with respect to reserved instance pricing

### Version 1.0.5 (2017-04-08)
 * Improved region detection of billing report line items

### Version 1.0.4 (2017-04-07)
 * Add support for AWS GovCloud (US) region
 * Better handling of new regions

### Version 1.0.3 (2017-04-04)
 * Fix issue preventing the configuration of a bucket residing in us-east-1

### Version 1.0.2 (2017-03-17)
 * Add indicator when billing reports are currently being processed
 * When selecting hourly interval, the hour of the datapoint is properly displayed in the tooltip
 * Support report names and paths with special characters, and reports which did not configure a "report path prefix"
 * Eliminate some resource leaks

### Version 1.0.1 (2017-02-23)
 * Initial release