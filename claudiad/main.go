// Copyright 2017 Applatix, Inc.
package main

import (
	"crypto/tls"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/applatix/claudia"
	"github.com/applatix/claudia/routers"
	"github.com/applatix/claudia/server"
	"github.com/applatix/claudia/userdb"
	"github.com/applatix/claudia/util"
	"github.com/gorilla/handlers"
	"github.com/urfave/cli"
)

// findAssets will attempt to return the path to assets dir based on the environment
func findAssets(assetsDir string) (string, error) {
	// Check for user supplied assets dir or if we are running in an installed environment (e.g. container)
	searchLocations := []string{assetsDir, claudia.ApplicationDir + "/assets", "assets"}
	for _, location := range searchLocations {
		if location == "" {
			continue
		}
		fi, err := os.Stat(location)
		if err == nil && fi.IsDir() {
			log.Printf("Assets located at %s", location)
			return location, nil
		}
	}
	err := errors.New("Failed to locate assets directory")
	return "", err
}

func openUserDatabase(userdbURL string, reinitialize bool) (*userdb.UserDatabase, error) {
	var datasource string
	if userdbURL == "" {
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
		datasource = fmt.Sprintf("postgres://%s@%s/%s?sslmode=disable", credentials, host, postgresDatabase)
	} else {
		datasource = userdbURL
	}
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
	tx, err := userDB.Begin()
	if err != nil {
		return nil, err
	}
	conf, err := tx.GetConfiguration()
	if err != nil {
		log.Println("Failed to retrieve configuration", err)
		tx.Rollback()
		return nil, err
	}
	tx.Commit()
	if reinitialize || conf == nil {
		err = userDB.Drop()
		if err != nil {
			log.Println("Failed to drop database", err)
			return nil, err
		}
		err = userDB.Init()
		if err != nil {
			log.Println("Failed to initialize database", err)
			return nil, err
		}
	}
	return userDB, nil
}

func redirectHandler(w http.ResponseWriter, r *http.Request) {
	target := "https://" + r.Host + r.URL.Path
	if len(r.URL.RawQuery) > 0 {
		target += "?" + r.URL.RawQuery
	}
	log.Printf("redirect to: %s", target)
	http.Redirect(w, r, target, http.StatusTemporaryRedirect)
}

func run(c *cli.Context) error {
	ingestdURL := c.String("ingestdURL")
	costdbURL := c.String("costdbURL")
	assets := c.String("assets")
	reinitialize := c.Bool("reinitialize")
	serverPort := c.Int("port")
	userdbURL := c.String("userdbURL")
	insecure := c.Bool("insecure")

	var err error
	var userDB *userdb.UserDatabase
	userDB, err = openUserDatabase(userdbURL, reinitialize)
	if err != nil {
		return err
	}
	assets, err = findAssets(assets)
	if err != nil {
		return err
	}
	svcContext, err := server.NewServerContext(ingestdURL, costdbURL, userDB, assets)
	if err != nil {
		return err
	}
	svcContext.CostDB.Wait()
	// creates the InfluxDB database if it doesn't exist
	err = svcContext.CostDB.CreateDatabase()
	if err != nil {
		return err
	}
	r := routers.InitializeRoutes(svcContext)
	handler := handlers.CORS(
		handlers.AllowCredentials(),
		handlers.AllowedHeaders([]string{"Content-Type"}),
	)(r)
	handler = handlers.LoggingHandler(os.Stdout, handler)
	handler = handlers.CompressHandler(handler)
	log.Println(fmt.Sprintf("Starting server on port %d (insecure: %t)", serverPort, insecure))
	if insecure {
		err = http.ListenAndServe(fmt.Sprintf(":%d", serverPort), handler)
	} else {
		if serverPort == 443 {
			go http.ListenAndServe(":80", http.HandlerFunc(redirectHandler))
		}
		tlsConf := &tls.Config{
			Certificates: []tls.Certificate{*svcContext.Certificate},
		}
		srv := &http.Server{
			Addr:      fmt.Sprintf(":%d", serverPort),
			Handler:   handler,
			TLSConfig: tlsConf,
		}
		srv.TLSConfig.GetCertificate = svcContext.GetCertificateFunc()
		err = srv.ListenAndServeTLS("", "")
		if err != nil {
			return err
		}
	}
	return err
}

func main() {
	util.RegisterStackDumper()
	util.StartStatsTicker(10 * time.Minute)
	app := cli.NewApp()
	app.Name = "claudia"
	app.Version = claudia.DisplayVersion
	app.Description = "Claudia Cost & Usage Analytics"
	app.Flags = []cli.Flag{
		cli.StringFlag{Name: "costdbURL", Value: claudia.CostDatabaseURL, Usage: "Cost Database URL"},
		cli.StringFlag{Name: "userdbURL", Value: "", Usage: "User Database URL"},
		cli.StringFlag{Name: "ingestdURL", Value: claudia.IngestdURL, Usage: "Ingest daemon URL"},
		cli.StringFlag{Name: "assets", Value: "", Usage: "Location of assets"},
		cli.BoolFlag{Name: "reinitialize", Usage: "Re-initialize the database"},
		cli.IntFlag{Name: "port", Value: claudia.ApplicationPort, Usage: "Server port"},
		cli.BoolFlag{Name: "insecure", Usage: "Run without https"},
	}
	app.Action = run
	app.Run(os.Args)
}
