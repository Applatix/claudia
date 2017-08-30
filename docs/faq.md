# Frequently Asked Questions

#### How frequently is the data updated?
AWS generates and delivers a new Cost and Usage report at least once a day to your billing bucket. Claudia checks once an hour for any new reports.

#### What instance type should I use to run the app?
Choosing the appropriate instance type depends on several factors, which include

* volume of data 
* retention length of the report
* cardinality of the data (which is the uniqueness of values in a column)
* performance expectations of queries

In general, you want enough memory to ensure your application performs well. Applatix recommends a minimum of 8 GB of memory. So instance types such as m4.large, m3.large, and t2.large are good starting points.

#### Why is there partially missing data on the last day of the month?
That's because the data for the last day of the month has not been "finalized" by AWS. The process of finalizing the report can take up to several days. Until AWS delivers this report to the billing bucket, there can be partially missing data for the last day of each month.

#### What is the difference between AWS Cost and Usage Reports vs. Detailed Billing Reports?
AWS Cost and Usage Reports is a newer billing report format (introduced December 2015) versus the classic Detailed Billing Reports (DBR) format (introduced December 2012).

Compared to the DBR, the Cost and Usage Reports provide additional columns, which enable more dimensions to explore the data. 

Most importantly, the Cost and Usage Reports now provide clear indications
on what instance pricing (reserved versus "on demand") was applied to an instance during usage. This enables you to better understand the effectiveness of your reserved instance purchases.

	