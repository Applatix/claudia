// Copyright 2017 Applatix, Inc.
package routers

import (
	"encoding/json"
	"net/http"

	"github.com/applatix/claudia/errors"
	"github.com/applatix/claudia/server"
	"github.com/applatix/claudia/userdb"
	"github.com/applatix/claudia/util"
	"github.com/gorilla/mux"
)

func configHandler(sc *server.ServerContext) func(http.ResponseWriter, *http.Request) {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, err := sc.SessionManager.ValidateSession(w, r)
		if err != nil {
			return
		}
		switch r.Method {
		case "GET":
			tx, err := sc.UserDB.Begin()
			if util.ErrorHandler(err, w) != nil {
				return
			}
			config, err := tx.GetConfiguration()
			if util.TXErrorHandler(err, tx, w) != nil {
				return
			}
			tx.Commit()
			config.PrivateKey = "" // do not return private key over the wire
			util.SuccessHandler(config, w)
		case "PUT":
			decoder := json.NewDecoder(r.Body)
			configUpdates := userdb.Configuration{}
			err = decoder.Decode(&configUpdates)
			if err != nil {
				err = errors.New(errors.CodeBadRequest, "Invalid config JSON")
				util.ErrorHandler(err, w)
				return
			}
			tx, err := sc.UserDB.Begin()
			if util.ErrorHandler(err, w) != nil {
				return
			}
			config, reload, err := tx.UpdateConfiguration(&configUpdates)
			if util.TXErrorHandler(err, tx, w) != nil {
				return
			}
			tx.Commit()
			if reload {
				sc.ReloadCertificate(config.PublicCertificate, config.PrivateKey)
			}
			config.PrivateKey = "" // do not return private key over the wire
			util.SuccessHandler(config, w)
		default:
			util.ErrorHandler(errors.Errorf(errors.CodeBadRequest, "Unsupported method %s", r.Method), w)
		}
	})
}

func accountHandler(sc *server.ServerContext) func(http.ResponseWriter, *http.Request) {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		si, err := sc.SessionManager.ValidateSession(w, r)
		if err != nil {
			return
		}
		switch r.Method {
		case "GET":
			tx, err := sc.UserDB.Begin()
			if util.ErrorHandler(err, w) != nil {
				return
			}
			user, err := tx.GetUserByID(si.UserID)
			if util.ErrorHandler(err, w) != nil {
				return
			}
			tx.Commit()
			util.SuccessHandler(user, w)
		case "PUT":
			decoder := json.NewDecoder(r.Body)
			user := userdb.User{}
			err = decoder.Decode(&user)
			if err != nil {
				err = errors.New(errors.CodeBadRequest, "Invalid user JSON")
				util.ErrorHandler(err, w)
				return
			}
			if user.ID == "" {
				user.ID = si.UserID
			} else if user.ID != si.UserID {
				err = errors.Errorf(errors.CodeForbidden, "Cannot change user ID %s to %s", si.UserID, user.ID)
				util.ErrorHandler(err, w)
				return
			}
			tx, err := sc.UserDB.Begin()
			if util.ErrorHandler(err, w) != nil {
				return
			}
			updatedUser, err := tx.UpdateUser(&user)
			if util.TXErrorHandler(err, tx, w) != nil {
				return
			}
			if user.CurrentPassword != user.Password {
				// Password was updated
				config, err := tx.RotateSessionKey()
				if util.TXErrorHandler(err, tx, w) != nil {
					return
				}
				sc.SessionManager.UpdateSessionKeys(config.SessionAuthKey, config.SessionCryptKey)
				err = sc.SessionManager.SetSession(updatedUser, w, r)
				if util.ErrorHandler(err, w) != nil {
					return
				}
			}
			tx.Commit()
			util.SuccessHandler(updatedUser, w)
		default:
			util.ErrorHandler(errors.Errorf(errors.CodeBadRequest, "Unsupported method %s", r.Method), w)
		}
	})
}

// InitializeRoutes initializes all HTTP endpoints and handlers
func InitializeRoutes(sc *server.ServerContext) *mux.Router {
	r := mux.NewRouter()
	// API endpoints
	r.HandleFunc("/v1/config", configHandler(sc)).Methods("GET", "PUT")
	r.HandleFunc("/v1/account", accountHandler(sc)).Methods("GET", "PUT")
	r.HandleFunc("/v1/cost", costHandler(sc))
	r.HandleFunc("/v1/count", countHandler(sc))
	r.HandleFunc("/v1/usage/{service}", usageHandler(sc))
	r.HandleFunc("/v1/usage/{service}/{metric}", usageHandler(sc))
	r.HandleFunc("/v1/usage", usageHandler(sc))
	r.HandleFunc("/v1/dimensions", rootDimensionHandler(sc))
	r.HandleFunc("/v1/dimensions/{dimension}", dimensionHandler(sc))
	r.HandleFunc("/v1/dimensions/{dimension}/{subdimension}", dimensionHandler(sc))
	r.HandleFunc("/v1/reports/{reportID}/status", reportStatusHandler(sc)).Methods("GET")
	r.HandleFunc("/v1/reports/{reportID}/buckets", reportBucketsHandler(sc)).Methods("GET", "POST")
	r.HandleFunc("/v1/reports/{reportID}/buckets/{bucketID}", reportBucketHandler(sc)).Methods("GET", "PUT", "DELETE")
	r.HandleFunc("/v1/reports/{reportID}/accounts", reportAccountsHandler(sc))
	r.HandleFunc("/v1/reports/{reportID}/accounts/{accountID}", reportAccountHandler(sc)).Methods("GET", "PUT")
	r.HandleFunc("/v1/reports/{reportID}", reportHandler(sc)).Methods("GET", "PUT", "DELETE")
	r.HandleFunc("/v1/reports", reportsHandler(sc)).Methods("GET", "POST")
	r.HandleFunc("/v1/auth/identity", authIdentityHandler(sc))
	r.HandleFunc("/v1/auth/login", authLoginHandler(sc)).Methods("POST")
	r.HandleFunc("/v1/auth/logout", authLogoutHandler(sc)).Methods("POST")
	// Serve static files
	r.PathPrefix("/assets/").Handler(http.StripPrefix("/assets/", http.FileServer(http.Dir(sc.AssetsDir+"/assets"))))
	// Redirect sub directories to the single page app URL (index.html)
	r.PathPrefix("/{subdir}/").Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Cache-Control", "max-age=0")
		http.ServeFile(w, r, sc.AssetsDir+"/index.html")
	}))
	// Serve root content in asset directory
	r.PathPrefix("/").Handler(http.StripPrefix("/", http.FileServer(http.Dir(sc.AssetsDir))))
	return r
}
