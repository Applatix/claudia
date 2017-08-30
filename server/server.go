// Copyright 2017 Applatix, Inc.
package server

import (
	"bytes"
	"crypto/tls"
	"log"
	"net/http"

	"github.com/applatix/claudia/costdb"
	"github.com/applatix/claudia/errors"
	"github.com/applatix/claudia/parser"
	"github.com/applatix/claudia/userdb"
)

// ServerContext provides context wrapper around the application server
type ServerContext struct {
	IngestdURL     string
	CostDB         *costdb.CostDatabase
	UserDB         *userdb.UserDatabase
	SessionManager *userdb.SessionManager
	AssetsDir      string
	Certificate    *tls.Certificate
}

// NewServerContext returns a new ServerContext instance
func NewServerContext(ingestdURL string, costdbURL string, userDB *userdb.UserDatabase, assetsDir string) (*ServerContext, error) {
	tx, err := userDB.Begin()
	if err != nil {
		return nil, err
	}
	conf, err := tx.GetConfiguration()
	if err != nil {
		tx.Rollback()
		return nil, err
	}
	tx.Commit()
	sMgr := userDB.NewSessionManager(conf.SessionAuthKey, conf.SessionCryptKey)
	costDB, err := costdb.NewCostDatabase(costdbURL)
	if err != nil {
		return nil, err
	}
	svcContext := ServerContext{
		IngestdURL:     ingestdURL,
		CostDB:         costDB,
		UserDB:         userDB,
		SessionManager: sMgr,
		AssetsDir:      assetsDir,
		Certificate:    nil,
	}
	svcContext.ReloadCertificate(conf.PublicCertificate, conf.PrivateKey)
	return &svcContext, nil
}

// NotifyUpdate will notify the ingestd service that changes have been made to the reports table
func (sc *ServerContext) NotifyUpdate() error {
	log.Printf("Notifying ingest daemon (%s) of report updates", sc.IngestdURL)
	resp, err := http.Post(sc.IngestdURL+"/v1/refresh", "application/json", bytes.NewBuffer([]byte("")))
	if err != nil {
		log.Printf("Failed to update ingestd of report updates: %q", err)
	} else {
		defer resp.Body.Close()
	}
	return errors.InternalError(err)
}

// GetDefaultReport returns a users default report
func (sc *ServerContext) GetDefaultReport(userID string) (*userdb.Report, error) {
	tx, err := sc.UserDB.Begin()
	if err != nil {
		return nil, errors.InternalError(err)
	}
	report, err := tx.GetUserDefaultReport(userID)
	if err != nil {
		tx.Rollback()
		return nil, err
	}
	tx.Commit()
	return report, nil
}

// GetUserReportStatus returns current statuses of report processing
func (sc *ServerContext) GetUserReportStatus(userID string, reportID string) ([]*costdb.IngestStatus, error) {
	tx, err := sc.UserDB.Begin()
	if err != nil {
		return nil, err
	}
	report, err := tx.GetUserReport(userID, reportID)
	if err != nil {
		tx.Rollback()
		return nil, err
	}
	tx.Commit()
	repCtx := sc.CostDB.NewCostReportContext(report.ID)
	return repCtx.GetReportIngestStatuses()
}

// GetDisplayNameAliases returns a mapping of account name aliases
func (sc *ServerContext) GetDisplayNameAliases(dimensionName string, report *userdb.Report) map[string]string {
	var aliases map[string]string
	switch dimensionName {
	case parser.ColumnUsageAccountID.APIName:
		// Substitute accountIDs with names
		aliases = make(map[string]string)
		for _, account := range report.Accounts {
			aliases[account.AWSAccountID] = account.Name
		}

	case parser.ColumnProductCode.APIName:
		// Substitute product codes with names
		products, err := sc.UserDB.GetProductAliases()
		if err != nil {
			log.Printf("Failed to lookup product codes: %s", err)
		} else {
			aliases = make(map[string]string)
			for k, product := range products {
				aliases[k] = product.Description
			}
		}
	case parser.ColumnRegion.APIName, parser.ColumnDataTransferDest.APIName, parser.ColumnDataTransferSource.APIName:
		aliases = make(map[string]string)
		for regionName, region := range parser.RegionMapping {
			aliases[regionName] = region.DisplayName
		}
	}
	return aliases
}

// ReloadCertificate reloads SSL certificate after an update
func (sc *ServerContext) ReloadCertificate(publicCertificate, privateKey string) error {
	log.Println("Reloading certificate")
	cert, err := tls.X509KeyPair([]byte(publicCertificate), []byte(privateKey))
	if err != nil {
		return errors.InternalError(err)
	}
	sc.Certificate = &cert
	return nil
}

// GetCertificateFunc will be set to tls.Config's GetCertificate member to retrieve current SSL certificate
func (sc *ServerContext) GetCertificateFunc() func(*tls.ClientHelloInfo) (*tls.Certificate, error) {
	return func(clientHello *tls.ClientHelloInfo) (*tls.Certificate, error) {
		return sc.Certificate, nil
	}
}
