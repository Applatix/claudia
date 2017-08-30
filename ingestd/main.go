// Copyright 2017 Applatix, Inc.
package main

import (
	"errors"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/applatix/claudia"
	"github.com/applatix/claudia/billingbucket"
	"github.com/applatix/claudia/costdb"
	"github.com/applatix/claudia/ingest"
	"github.com/applatix/claudia/userdb"
	"github.com/applatix/claudia/util"
	"github.com/urfave/cli"
)

func fetchReports(c *cli.Context) error {
	reportDir := c.String("reportDir")
	awsAccessKeyID := c.String("awsAccessKeyID")
	awsSecretAccessKey := c.String("awsSecretAccessKey")
	s3bucket := c.String("bucket")
	reportPathPrefix := c.String("reportPathPrefix")
	_ = c.BoolT("purgePrevious")

	if reportDir == "" {
		return errors.New("Local report dir unspecified")
	}
	if s3bucket == "" || reportPathPrefix == "" {
		return errors.New("S3 bucket and report path must be specified")
	}

	region, err := billingbucket.GetBucketRegion(s3bucket)
	if err != nil {
		return err
	}

	_, err = billingbucket.NewAWSBillingBucket(awsAccessKeyID, awsSecretAccessKey, s3bucket, region, reportPathPrefix)
	if err != nil {
		log.Printf("Could not access billing bucket: %s", err)
		return err
	}
	//return ingest.DownloadReports(billbuck, reportDir, nil, purgePrevious)
	return errors.New("Fetch reports not yet implemented")
}

func processReports(c *cli.Context) error {
	reportDir := c.String("reportDir")
	costDBURL := c.String("costdb")
	dropDatabase := c.Bool("dropDb")
	reportID := c.String("reportID")

	if reportID == "" {
		return errors.New("reportID unspecified")
	}
	if reportDir == "" {
		return errors.New("Local report dir unspecified")
	}

	costDB, err := costdb.NewCostDatabase(costDBURL)
	if err != nil {
		return err
	}
	defer costDB.Close()
	if dropDatabase {
		costDB.DropDatabase()
	}
	_ = costDB.NewCostReportContext(reportID)
	return errors.New("Process reports not yet implemented")
}

func openUserDatabase() (*userdb.UserDatabase, error) {
	var datasource string
	// PostgreSQL. TODO: enable ssl
	host := os.Getenv("USERDB_HOST")
	if host == "" {
		host = "userdb:5432"
	}
	postgresUser := os.Getenv("POSTGRES_USER")
	if postgresUser == "" {
		postgresUser = "postgres"
	}
	postgresPassword := os.Getenv("POSTGRES_PASSWORD")
	postgresDatabase := os.Getenv("POSTGRES_DB")
	if postgresDatabase == "" {
		postgresDatabase = postgresUser
	}
	var credentials string
	if postgresPassword == "" {
		credentials = postgresUser
	} else {
		credentials = fmt.Sprintf("%s:%s", postgresUser, postgresPassword)
	}
	//datasource := fmt.Sprintf("postgres://%s:%s@%s/?sslmode=verify-full", username, password, host)
	datasource = fmt.Sprintf("postgres://%s@%s/%s?sslmode=disable", credentials, host, postgresDatabase)

	var userDB *userdb.UserDatabase
	var err error
	for i := 1; i <= 10; i++ {
		userDB, err = userdb.Open(datasource)
		if err == nil {
			break
		} else {
			log.Println("Failed to open connection. Retrying", err)
			time.Sleep(5 * time.Second)
		}
	}
	if err != nil {
		return nil, err
	}
	return userDB, nil
}

func dropDatabase(c *cli.Context) error {
	costDBURL := c.String("costdb")
	costDB, err := costdb.NewCostDatabase(costDBURL)
	if err != nil {
		return err
	}
	defer costDB.Close()
	return costDB.DropDatabase()
}

func run(c *cli.Context) error {
	reportDir := c.String("reportDir")
	costDBURL := c.String("costdb")
	workers := c.Int("workers")
	port := c.Int("port")
	userDB, err := openUserDatabase()
	if err != nil {
		return err
	}
	isc, err := ingest.NewIngestSvcContext(userDB, costDBURL, reportDir, workers)
	if err != nil {
		return err
	}
	return isc.Run(port)
}

func main() {
	util.RegisterStackDumper()
	util.StartStatsTicker(10 * time.Minute)
	app := cli.NewApp()
	app.Name = "ingestd"
	app.Version = claudia.DisplayVersion

	reportDirFlag := cli.StringFlag{Name: "reportDir", Value: claudia.ApplicationDir + "/reports", Usage: "Local directory to download and process reports"}
	costDBFlag := cli.StringFlag{Name: "costdb", Value: claudia.CostDatabaseURL, Usage: "URL of cost database"}

	app.Commands = []cli.Command{
		{
			Name:  "fetch",
			Usage: "Fetch the cost and usage reports to local directory",
			Flags: []cli.Flag{
				reportDirFlag,
				cli.StringFlag{Name: "bucket", Value: "", Usage: "Name of S3 bucket containing the reports"},
				cli.StringFlag{Name: "awsAccessKeyId", Value: "", Usage: "AWS access key ID"},
				cli.StringFlag{Name: "awsSecretAccessKey", Value: "", Usage: "AWS secret access key"},
				cli.StringFlag{Name: "reportPathPrefix", Value: "", Usage: "Report path to reports (excluding date range)"},
				cli.BoolTFlag{Name: "purgePrevious", Usage: "Purge any previous outdated reports"},
			},
			Action: fetchReports,
		},
		{
			Name:  "process",
			Usage: "Process reports and ingest to cost database",
			Flags: []cli.Flag{
				reportDirFlag,
				costDBFlag,
				cli.BoolFlag{Name: "dropDb", Usage: "Drop the database before importing"},
			},
			Action: processReports,
		},
		{
			Name:  "drop",
			Usage: "Drop the cost and usage database",
			Flags: []cli.Flag{
				costDBFlag,
			},
			Action: dropDatabase,
		},
		{
			Name:  "run",
			Usage: "Run as a service which periodically polls and fetches/processes reports",
			Flags: []cli.Flag{
				reportDirFlag,
				costDBFlag,
				cli.IntFlag{Name: "workers", Value: claudia.IngestdWorkers, Usage: "Number of concurrent ingest workers"},
				cli.IntFlag{Name: "port", Value: claudia.IngestdPort, Usage: "Port to run on"},
			},
			Action: run,
		},
	}
	app.Run(os.Args)
}
