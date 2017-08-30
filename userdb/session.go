// Copyright 2017 Applatix, Inc.
package userdb

import (
	"log"
	"net/http"

	"github.com/applatix/claudia/errors"
	"github.com/applatix/claudia/util"
	"github.com/gorilla/sessions"
)

// SessionManager manages session state for API clients
type SessionManager struct {
	Store  *sessions.CookieStore
	userDB *UserDatabase
}

// SessionInfo is the object stored in the client's encrypted session cookie
type SessionInfo struct {
	UserID   string
	Username string
}

// ValidateSession will validate the session supplied by the cookie store and return session information about the current profile
func (s *SessionManager) ValidateSession(w http.ResponseWriter, r *http.Request) (*SessionInfo, error) {
	session, err := s.Store.Get(r, "session")
	if err != nil {
		err = errors.Errorf(errors.CodeUnauthorized, "Previous session no longer valid: %s", err)
		log.Println(err)
		s.DeleteSession(w, r)
		util.ErrorHandler(err, w)
		return nil, err
	}
	if session.IsNew {
		err = errors.Errorf(errors.CodeUnauthorized, "No valid session")
		log.Println(err)
		util.ErrorHandler(err, w)
		return nil, err
	}
	var si SessionInfo
	var exists bool
	username, exists := session.Values["username"]
	if !exists || username.(string) == "" {
		err = errors.Errorf(errors.CodeUnauthorized, "Existing session invalid: (username: %s)", si.Username)
		log.Println(err)
		s.DeleteSession(w, r)
		util.ErrorHandler(err, w)
		return nil, err
	}
	si.Username = username.(string)
	userID, exists := session.Values["user_id"]
	if !exists || userID == nil || userID.(string) == "" {
		err = errors.Errorf(errors.CodeUnauthorized, "Existing session invalid: (id: %s)", si.UserID)
		log.Println(err)
		s.DeleteSession(w, r)
		util.ErrorHandler(err, w)
		return nil, err
	}
	si.UserID = userID.(string)
	return &si, nil
}

// SetSession will write the session in the http response writter
func (s *SessionManager) SetSession(user *User, w http.ResponseWriter, r *http.Request) error {
	log.Printf("Saving new session for %s (id: %s)", user.Username, user.ID)
	if user.Username == "" || user.PasswordHash == nil || user.ID == "" {
		// Internal sanity check to ensure we are provided valid session
		return errors.New(errors.CodeInternal, "incomplete user session information")
	}
	session, _ := s.Store.Get(r, "session")
	session.Values["user_id"] = user.ID
	session.Values["username"] = user.Username
	err := session.Save(r, w)
	if err != nil {
		log.Printf("Failed to save session: %s", err)
		return errors.InternalError(err)
	}
	log.Printf("Saved new session for %s (id: %s)", user.Username, user.ID)
	return nil
}

// DeleteSession will delete the user's session
func (s *SessionManager) DeleteSession(w http.ResponseWriter, r *http.Request) {
	log.Println("Deleting session")
	session, _ := s.Store.Get(r, "session")
	delete(session.Values, "user_id")
	delete(session.Values, "username")
	session.Options.MaxAge = -1
	session.Save(r, w)
}

// UpdateSessionKeys updates the session keys used by the session manager.
// Calling this invalidates all previously issued session tokens site-wide
func (s *SessionManager) UpdateSessionKeys(authKey, cryptKey []byte) {
	s.Store = sessions.NewCookieStore(authKey, cryptKey)
}

// NewSessionManager returns a new session manager instance from the user database context
func (db *UserDatabase) NewSessionManager(authKey, cryptKey []byte) *SessionManager {
	cookieStore := sessions.NewCookieStore(authKey, cryptKey)
	return &SessionManager{cookieStore, db}
}
