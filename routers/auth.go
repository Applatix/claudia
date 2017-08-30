// Copyright 2017 Applatix, Inc.
package routers

import (
	"encoding/json"
	"net/http"

	"github.com/applatix/claudia/server"
	"github.com/applatix/claudia/userdb"
	"github.com/applatix/claudia/util"
)

// authIdentityHandler is a HTTP handler which simply validates & decodes the session cooke to return current logged in user
func authIdentityHandler(sc *server.ServerContext) func(http.ResponseWriter, *http.Request) {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		si, err := sc.SessionManager.ValidateSession(w, r)
		if err != nil {
			return
		}
		identity := make(map[string]interface{}, 0)
		identity["username"] = si.Username
		util.SuccessHandler(identity, w)
	})
}

// authIdentityHandler is a HTTP handler which authenticates a user and sets a new session cookie
func authLoginHandler(sc *server.ServerContext) func(http.ResponseWriter, *http.Request) {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		decoder := json.NewDecoder(r.Body)
		var reqUser userdb.User
		err := decoder.Decode(&reqUser)
		if util.ErrorHandler(err, w) != nil {
			return
		}
		defer r.Body.Close()
		dbUser, err := sc.UserDB.AuthenticateUser(reqUser.Username, reqUser.Password)
		if util.ErrorHandler(err, w) != nil {
			return
		}
		err = sc.SessionManager.SetSession(dbUser, w, r)
		if util.ErrorHandler(err, w) != nil {
			return
		}
		util.SuccessHandler(dbUser, w)
	})
}

// authIdentityHandler is a HTTP handler which will delete and expire the user session cookie
func authLogoutHandler(sc *server.ServerContext) func(http.ResponseWriter, *http.Request) {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sc.SessionManager.DeleteSession(w, r)
		util.SuccessHandler(nil, w)
	})
}
