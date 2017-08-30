// Copyright 2017 Applatix, Inc.
package userdb

import (
	"crypto/tls"
	"database/sql"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strings"
	"time"
	"unicode"

	"github.com/applatix/claudia"
	"github.com/applatix/claudia/errors"
	"github.com/applatix/claudia/util"
	"github.com/gorilla/securecookie"
	"github.com/jmoiron/sqlx"
	// load the postgres driver for sqlx
	_ "github.com/lib/pq"
	"golang.org/x/crypto/bcrypt"
)

// UserDatabase is a wrapper around SQL DB instance and provides querying capabilities
type UserDatabase struct {
	*sqlx.DB
}

// Tx is a wrapper around Tx to provide querying interfaces against the user database
type Tx struct {
	*sqlx.Tx
}

// Configuration is the global configuration for this app. Maps to the 'configuration' table
type Configuration struct {
	ID                int    `db:"id" json:"-"`
	SchemaVersion     int    `db:"schema_version" json:"-"`
	SessionAuthKey    []byte `db:"session_auth_key" json:"-"`
	SessionCryptKey   []byte `db:"session_crypt_key" json:"-"`
	PrivateKey        string `db:"private_key" json:"private_key,omitempty"`
	PublicCertificate string `db:"public_certificate" json:"public_certificate"`
	EULAAccepted      bool   `db:"eula_accepted" json:"eula_accepted"`
}

// User is an application user. Maps to the 'appuser' table
type User struct {
	ID              string    `db:"id" json:"id"`
	CTime           time.Time `db:"ctime" json:"ctime"`
	MTime           time.Time `db:"mtime" json:"mtime"`
	Username        string    `db:"username" json:"username"`
	CurrentPassword string    `json:"current_password,omitempty"`
	Password        string    `json:"password,omitempty"`
	PasswordHash    []byte    `db:"password_hash" json:"-"`
}

// Report is the struct representing a cost & usage report. Maps to the 'report' table
type Report struct {
	ID            string               `db:"id" json:"id"`
	CTime         time.Time            `db:"ctime" json:"ctime"`
	MTime         time.Time            `db:"mtime" json:"mtime"`
	Status        claudia.ReportStatus `db:"status" json:"status"`
	StatusDetail  string               `db:"status_detail" json:"status_detail"`
	OwnerUserID   string               `db:"owner_user_id" json:"owner_user_id"`
	ReportName    string               `db:"report_name" json:"report_name"`
	RetentionDays int                  `db:"retention_days" json:"retention_days"`
	Buckets       []*Bucket            `json:"buckets"`
	Accounts      []*AWSAccountInfo    `json:"accounts"`
}

// AWSAccountInfo represents an AWS account mapping of ID to name in a cost & usage report
type AWSAccountInfo struct {
	AWSAccountID string `db:"aws_account_id" json:"aws_account_id"`
	Name         string `db:"name" json:"name"`
	ReportID     string `db:"report_id" json:"-"`
}

// Bucket represents a billing bucket associated with a cost & usage report
type Bucket struct {
	ID                 string    `db:"id" json:"id"`
	ReportID           string    `db:"report_id" json:"report_id"`
	CTime              time.Time `db:"ctime" json:"ctime"`
	Bucketname         string    `db:"bucketname" json:"bucketname"`
	Region             string    `db:"region" json:"region"`
	ReportPath         string    `db:"report_path" json:"report_path"`
	AWSAccessKeyID     string    `db:"aws_access_key_id" json:"aws_access_key_id"`
	AWSSecretAccessKey string    `db:"aws_secret_access_key" json:"aws_secret_access_key,omitempty"`
}

// AWSProductInfo represents information about a AWS Marketplace product
type AWSProductInfo struct {
	ProductCode string `db:"product_code" json:"product_code"`
	Name        string `db:"name" json:"name"`
	Description string `db:"description" json:"description"`
}

// Open returns a DB reference for a data source.
func Open(datasource string) (*UserDatabase, error) {
	driver := strings.SplitN(datasource, ":", 2)[0]
	log.Printf("Connecting to %s", datasource)
	db, err := sqlx.Connect(driver, datasource)
	if err != nil {
		return nil, errors.InternalError(err)
	}
	userDB := UserDatabase{db}
	return &userDB, nil
}

// GetBucket returns the bucket in this report's bucket list with the given name and report path
func (r *Report) GetBucket(bucketname, reportPath string) *Bucket {
	for _, bucket := range r.Buckets {
		if bucket.Bucketname == bucketname && bucket.ReportPath == reportPath {
			return bucket
		}
	}
	return nil
}

// ETag returns an HTTP ETag string to enable client side caching of report results
func (r *Report) ETag() string {
	return fmt.Sprintf("%s/%s", r.ID, r.MTime.UTC().String())
}

// Drop database
func (db *UserDatabase) Drop() error {
	db.MustExec("DROP SCHEMA public CASCADE;")
	db.MustExec("CREATE SCHEMA public;")
	db.MustExec("GRANT ALL ON SCHEMA public TO postgres;")
	db.MustExec("GRANT ALL ON SCHEMA public TO public;")
	return nil
}

// getDefaultPassword returns a default password when initializing the database schema.
// If we are running as an AMI, the this will be the instance id of the image.
// In all other cases, will be the default password
func getDefaultPassword() string {
	if _, err := os.Stat("/proc/xen"); os.IsNotExist(err) {
		log.Println("Running outside AWS. Using common default password.")
		return claudia.ApplicationAdminDefaultPassword
	}
	if _, err := os.Stat("/var/run/secrets/kubernetes.io"); err == nil {
		// NOTE: the presence of this file relies on the k8s ServiceAccount plugin
		// being active, which is true in most distributions
		log.Println("Running in AWS kubernetes cluster. Using common default password.")
		return claudia.ApplicationAdminDefaultPassword
	}
	resp, err := http.Get("http://169.254.169.254/latest/meta-data/instance-id")
	if err != nil {
		log.Printf("Error retrieving instance id (%s). Using common default password.", err)
		return claudia.ApplicationAdminDefaultPassword
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Printf("Error retrieving reading HTTP response (%s). Using common default password.", err)
		return claudia.ApplicationAdminDefaultPassword
	}
	log.Println("Running as AWS AMI. Using instance ID as password.")
	return strings.TrimSpace(string(body))
}

// Init initializes or upgrade the database schema
func (db *UserDatabase) Init() error {
	var err error
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	for _, stmt := range schemaV1 {
		log.Println(stmt)
		tx.MustExec(stmt)
	}
	defaultPassword := getDefaultPassword()
	_, err = tx.CreateUser(claudia.ApplicationAdminUsername, defaultPassword)
	if err != nil {
		return err
	}
	sessionAuthKey := securecookie.GenerateRandomKey(32)
	sessionCryptKey := securecookie.GenerateRandomKey(32)
	crt, key := util.GenerateSelfSignedCert()
	tx.MustExec("INSERT INTO configuration (schema_version, session_auth_key, session_crypt_key, private_key, public_certificate) VALUES ($1, $2, $3, $4, $5)",
		SchemaVersion, sessionAuthKey, sessionCryptKey, key, crt)
	tx.Commit()
	tx, err = db.Begin()
	if err != nil {
		return err
	}
	conf, err := tx.GetConfiguration()
	if err != nil {
		tx.Rollback()
		return err
	}
	tx.Commit()
	log.Printf("Successfully initialized database (schema: %d)", conf.SchemaVersion)
	return nil
}

// Wait blocks until the database is ready
func (db *UserDatabase) Wait() {
	log.Println("Waiting for user db to become ready")
	for {
		tx, err := db.Begin()
		if err == nil {
			conf, err := tx.GetConfiguration()
			if err == nil && conf != nil && conf.SchemaVersion == SchemaVersion {
				tx.Commit()
				log.Println("User db is ready")
				return
			}
			tx.Rollback()
		}
		time.Sleep(3 * time.Second)
	}
}

// Begin starts and returns a new transaction.
func (db *UserDatabase) Begin() (*Tx, error) {
	tx, err := db.Beginx()
	if err != nil {
		return nil, errors.InternalError(err)
	}
	return &Tx{tx}, nil
}

// GetProductAliases returns a mapping of product codes to its product info
func (db *UserDatabase) GetProductAliases() (map[string]*AWSProductInfo, error) {
	tx, err := db.Begin()
	if err != nil {
		return nil, err
	}
	products, err := tx.GetProducts()
	if err != nil {
		tx.Rollback()
		return nil, err
	}
	tx.Commit()
	aliases := make(map[string]*AWSProductInfo)
	for _, product := range products {
		aliases[product.ProductCode] = product
	}
	return aliases, nil
}

// GetConfiguration returns system configuration. If system is not configured returns nil without error
func (tx *Tx) GetConfiguration() (*Configuration, error) {
	var conf Configuration
	err := tx.Get(&conf, "SELECT * FROM configuration;")
	if err != nil {
		if strings.Contains(err.Error(), "does not exist") {
			// pq: relation "configuration" does not exist
			// TODO: find better detection when table does not exist
			return nil, nil
		}
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, errors.InternalError(err)
	}
	return &conf, nil
}

// UpdateConfiguration updates system wide configuration (e.g. EULA, public certificate, private key)
// Returns the updated configuration, and a boolean indicating if the certificates should be reloaded
func (tx *Tx) UpdateConfiguration(configUpdates *Configuration) (*Configuration, bool, error) {
	updates := make(map[string]interface{})
	reload := false
	if configUpdates.PrivateKey != "" || configUpdates.PublicCertificate != "" {
		origConfig, err := tx.GetConfiguration()
		if err != nil {
			return nil, false, err
		}
		privateKey := origConfig.PrivateKey
		publicCert := origConfig.PublicCertificate
		if configUpdates.PrivateKey != "" && configUpdates.PrivateKey != privateKey {
			privateKey = configUpdates.PrivateKey
			reload = true
		}
		if configUpdates.PublicCertificate != "" && configUpdates.PublicCertificate != publicCert {
			publicCert = configUpdates.PublicCertificate
			reload = true
		}
		// Verify the key pair is valid before storing to database
		_, err = tls.X509KeyPair([]byte(publicCert), []byte(privateKey))
		if err != nil {
			return nil, false, errors.Errorf(errors.CodeBadRequest, "Unable to update certificate: %s", err)
		}
		updates["private_key"] = privateKey
		updates["public_certificate"] = publicCert
	}
	if configUpdates.EULAAccepted {
		updates["eula_accepted"] = true
	}
	if len(configUpdates.SessionAuthKey) > 0 {
		updates["session_auth_key"] = configUpdates.SessionAuthKey
	}
	if len(configUpdates.SessionCryptKey) > 0 {
		updates["session_crypt_key"] = configUpdates.SessionCryptKey
	}
	if len(updates) == 0 {
		return nil, false, errors.New(errors.CodeBadRequest, "No valid configuration updates supplied")
	}
	assignments := make([]string, len(updates))
	values := make([]interface{}, len(updates))
	i := 0
	for colName, value := range updates {
		assignments[i] = fmt.Sprintf("%s = $%d", colName, i+1)
		values[i] = value
		i++
	}
	var config Configuration
	query := fmt.Sprintf("UPDATE configuration SET %s RETURNING *;", strings.Join(assignments, ", "))
	err := tx.Get(&config, query, values...)
	if err != nil {
		return nil, false, errors.InternalError(err)
	}
	return &config, reload, nil
}

// RotateSessionKey rotates the system-wide session key and updates the configuration table with the new keys
// This is called upon a password update to invalidate sessions of all other logins.
// NOTE: this only works because we have a single user (admin) and need redesign upon a multi-user site
func (tx *Tx) RotateSessionKey() (*Configuration, error) {
	config := &Configuration{
		SessionAuthKey:  securecookie.GenerateRandomKey(32),
		SessionCryptKey: securecookie.GenerateRandomKey(32),
	}
	config, _, err := tx.UpdateConfiguration(config)
	return config, err
}

// CreateUser creates a new user.
// Returns an error if user is invalid or the tx fails.
func (tx *Tx) CreateUser(username, password string) (*User, error) {
	log.Printf("Creating user %s", username)
	// Validate the input.
	if username == "" {
		return nil, errors.New(errors.CodeBadRequest, "Username required")
	}
	if password == "" {
		return nil, errors.New(errors.CodeBadRequest, "Password required")
	}
	err := verifyPasswordStrength(password)
	if err != nil {
		return nil, err
	}
	passwordHash, err := createPasswordHash(password)
	if err != nil {
		return nil, err
	}
	var user User
	err = tx.Get(&user, "INSERT INTO appuser (username, password_hash) VALUES ($1, $2) RETURNING *;", username, passwordHash)
	if err != nil {
		return nil, errors.InternalError(err)
	}
	log.Printf("Created user %s with id %s", user.Username, user.ID)
	return &user, nil
}

// createPasswordHash generates a password hash for storage in the database
func createPasswordHash(password string) ([]byte, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, errors.InternalError(err)
	}
	return hash, err
}

// compareHashAndPassword compares a password hash with a supplied password and returns true if it matches
func compareHashAndPassword(passwordHash []byte, password string) bool {
	err := bcrypt.CompareHashAndPassword(passwordHash, []byte(password))
	return err == nil
}

// verifyPasswordStrength returns true if the password meets the strength requirements
// For now, this means it meets a minimum length and does not contain any invalid characters
func verifyPasswordStrength(password string) error {
	const minPasswordLength = 8
	if len(password) < minPasswordLength {
		return errors.Errorf(errors.CodeBadRequest, "Password length must at least %d characters", minPasswordLength)
	}
	for _, s := range password {
		if !unicode.IsLetter(s) && !unicode.IsNumber(s) && !unicode.IsPunct(s) && !unicode.IsSymbol(s) && s != ' ' {
			return errors.New(errors.CodeBadRequest, "Passwords must only contain letters, numbers, punctuation, and symbols")
		}
	}
	return nil
}

// UpdateUser updates user information.
// Returns an error if user is invalid or the tx fails.
func (tx *Tx) UpdateUser(u *User) (*User, error) {
	log.Printf("Updating user %s", u.ID)
	updates := make(map[string]interface{}, 0)
	dbUser, err := tx.GetUserByID(u.ID)
	if err != nil {
		return nil, err
	}
	if u.Username != "" && u.Username != dbUser.Username {
		if dbUser.Username == claudia.ApplicationAdminUsername {
			return nil, errors.New(errors.CodeForbidden, "Admin username cannot be changed")
		}
		updates["username"] = u.Username
	}
	if u.Password != "" {
		err := verifyPasswordStrength(u.Password)
		if err != nil {
			return nil, err
		}
		if u.CurrentPassword == "" {
			return nil, errors.New(errors.CodeUnauthorized, "Current password required when updating password")
		}
		validPw := compareHashAndPassword(dbUser.PasswordHash, u.CurrentPassword)
		if !validPw {
			return nil, errors.New(errors.CodeUnauthorized, "Current password incorrect")
		}
		passwordHash, err := createPasswordHash(u.Password)
		if err != nil {
			return nil, err
		}
		updates["password_hash"] = passwordHash
	}
	if len(updates) == 0 {
		log.Println("Update user invoked but no updates were necessary/applied")
		return dbUser, nil
	}
	updates["mtime"] = time.Now().UTC()
	assignments := make([]string, len(updates))
	values := make([]interface{}, len(updates))
	i := 0
	for colName, value := range updates {
		assignments[i] = fmt.Sprintf("%s = $%d", colName, i+1)
		values[i] = value
		i++
	}
	query := fmt.Sprintf("UPDATE appuser SET %s WHERE id = '%s' RETURNING *;", strings.Join(assignments, ", "), u.ID)
	log.Println(query)
	var user User
	err = tx.Get(&user, query, values...)
	if err != nil {
		return nil, errors.InternalError(err)
	}
	return &user, nil
}

// GetUserByUsername retrieves a User by username
func (tx *Tx) GetUserByUsername(username string) (*User, error) {
	var user User
	err := tx.Get(&user, "SELECT * FROM appuser WHERE username = $1;", username)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, errors.Errorf(errors.CodeBadRequest, "User '%s' does not exist", username)
		}
		return nil, errors.InternalError(err)
	}
	return &user, err
}

// GetUserByID retrieves a User by ID
func (tx *Tx) GetUserByID(userID string) (*User, error) {
	var user User
	err := tx.Get(&user, "SELECT * FROM appuser WHERE id = $1;", userID)
	if err != nil {
		return nil, errors.InternalError(err)
	}
	return &user, err
}

// AuthenticateUser check credentials for a user.
// Returns an error if user is invalid or the tx fails.
func (db *UserDatabase) AuthenticateUser(username, password string) (*User, error) {
	tx, err := db.Begin()
	if err != nil {
		return nil, errors.InternalError(err)
	}
	user, err := tx.GetUserByUsername(username)
	if err != nil {
		tx.Rollback()
		if apiErr, ok := err.(errors.APIError); ok && apiErr.Code() == errors.CodeBadRequest {
			return nil, errors.New(errors.CodeUnauthorized, "Invalid username or password")
		}
		return nil, err
	}
	tx.Commit()
	validPw := compareHashAndPassword(user.PasswordHash, password)
	if !validPw {
		return nil, errors.New(errors.CodeUnauthorized, "Invalid username or password")
	}
	return user, nil
}

const selectReportsQuery = "SELECT * FROM report r"

func (tx *Tx) getReportsHelper(query string, args ...interface{}) ([]*Report, error) {
	reports := []*Report{}
	err := tx.Select(&reports, query, args...)
	if err != nil {
		return nil, errors.InternalError(err)
	}
	for _, report := range reports {
		report.Buckets, err = tx.GetReportBuckets(report.ID)
		if err != nil {
			return nil, err
		}
		report.Accounts, err = tx.GetAWSAccounts(report.ID)
		if err != nil {
			return nil, err
		}
	}
	return reports, nil
}

// GetAWSAccounts retrieves all AWS accounts associated with a report
func (tx *Tx) GetAWSAccounts(reportID string) ([]*AWSAccountInfo, error) {
	awsAccounts := []*AWSAccountInfo{}
	err := tx.Select(&awsAccounts, "SELECT * FROM aws_account WHERE report_id = $1", reportID)
	if err != nil {
		return nil, errors.InternalError(err)
	}
	return awsAccounts, nil
}

// GetReports retrieves the report owned by the user
func (tx *Tx) GetReports() ([]*Report, error) {
	return tx.getReportsHelper(selectReportsQuery)
}

// GetUserReports retrieves all reports owned by the user
func (tx *Tx) GetUserReports(userID string) ([]*Report, error) {
	return tx.getReportsHelper(selectReportsQuery+" WHERE r.owner_user_id = $1;", userID)
}

// GetUserReport retrieves the specified report owned by the user
func (tx *Tx) GetUserReport(userID string, reportID string) (*Report, error) {
	reports, err := tx.getReportsHelper(selectReportsQuery+" WHERE r.owner_user_id = $1 AND r.id = $2;", userID, reportID)
	if err != nil {
		return nil, err
	}
	if len(reports) == 0 {
		return nil, errors.Errorf(errors.CodeNotFound, "Report %s not found", reportID)
	}
	return reports[0], nil
}

// GetUserDefaultReport retrieves the default report owned by the user. Returns nil if no reports are configured
// Since the database currently has a constraint of one report per user, this simply returns the first row (for now).
func (tx *Tx) GetUserDefaultReport(userID string) (*Report, error) {
	reports, err := tx.GetUserReports(userID)
	if err != nil {
		return nil, err
	}
	if len(reports) == 0 {
		return nil, errors.New(errors.CodeNotFound, "No reports configured")
	}
	return reports[0], nil
}

// CreateUserReport create the report owned by the user
func (tx *Tx) CreateUserReport(userID string, r *Report) (string, error) {
	log.Printf("Creating report under user %s", userID)
	var reportID string
	var reportName string
	var retentionDays int
	if r.ReportName == "" {
		reportName = "default"
	} else {
		reportName = r.ReportName
	}
	if r.RetentionDays == 0 {
		retentionDays = claudia.ReportDefaultRetentionDays
	} else if r.RetentionDays < 7 {
		// Prevents influxdb error: "retention policy duration must be greater than the shard duration"
		return "", errors.New(errors.CodeBadRequest, "Retention period must be at least 7 days")
	} else {
		retentionDays = r.RetentionDays
	}
	err := tx.QueryRow("INSERT INTO report (owner_user_id, report_name, retention_days, status, status_detail) VALUES ($1, $2, $3, $4, $5) RETURNING id",
		userID, reportName, retentionDays, string(claudia.ReportStatusCurrent), "").Scan(&reportID)
	if err != nil {
		// If we violate the constraint, report_owner_user_id_key, user is attempting to create multiple reports
		// pq: duplicate key value violates unique constraint \"report_owner_user_id_key\""
		if strings.Contains(err.Error(), "report_owner_user_id_key") {
			return "", errors.New(errors.CodeForbidden, "multiple reports per user is unsupported at this time")
		}
		return "", errors.InternalError(err)
	}
	log.Printf("Created reportID: %s under user %s", reportID, userID)
	return reportID, nil
}

// UpdateUserReport applies updates to a report
func (tx *Tx) UpdateUserReport(r *Report) error {
	log.Printf("Updating report %s", r.ID)
	updates := make(map[string]interface{}, 0)
	if r.ReportName != "" {
		updates["report_name"] = r.ReportName
	}
	if r.RetentionDays != 0 {
		if r.RetentionDays < 7 {
			// Prevents influxdb error: "retention policy duration must be greater than the shard duration"
			return errors.New(errors.CodeBadRequest, "Retention period must be at least 7 days")
		}
		updates["retention_days"] = r.RetentionDays
	}
	if !r.MTime.IsZero() {
		updates["mtime"] = r.MTime.UTC()
	} else if len(updates) > 0 {
		updates["mtime"] = time.Now().UTC()
	}
	if r.Status != "" {
		updates["status"] = string(r.Status)
	}
	if r.StatusDetail != "" {
		updates["status_detail"] = r.StatusDetail
	}
	if r.Status == claudia.ReportStatusCurrent {
		// If status is current, there's no detail necessary
		updates["status_detail"] = ""
	}
	if len(updates) > 0 {
		assignments := make([]string, len(updates))
		values := make([]interface{}, len(updates))
		i := 0
		for colName, value := range updates {
			assignments[i] = fmt.Sprintf("%s = $%d", colName, i+1)
			values[i] = value
			i++
		}
		query := fmt.Sprintf("UPDATE report SET %s WHERE id = '%s';", strings.Join(assignments, ", "), r.ID)
		log.Println(query)
		_, err := tx.Exec(query, values...)
		if err != nil {
			return errors.InternalError(err)
		}
	}
	return nil
}

// UpdateUserReportMtime updates the report's modification time
func (tx *Tx) UpdateUserReportMtime(reportID string) error {
	return tx.UpdateUserReport(&Report{ID: reportID, MTime: time.Now().UTC()})
}

// UpdateUserReportStatus updates the report's status and mtime
func (tx *Tx) UpdateUserReportStatus(reportID string, status claudia.ReportStatus, statusDetail string) error {
	log.Printf("Updating report %s status to: %s: %s", reportID, status, statusDetail)
	return tx.UpdateUserReport(&Report{ID: reportID, MTime: time.Now().UTC(), Status: status, StatusDetail: statusDetail})
}

// DeleteUserReport deletes the report owned by the user
func (tx *Tx) DeleteUserReport(userID string, reportID string) error {
	res, err := tx.Exec("DELETE FROM report WHERE owner_user_id = $1 AND id = $2;", userID, reportID)
	if err != nil {
		return errors.InternalError(err)
	}
	affected, _ := res.RowsAffected()
	log.Printf("Deleted %d reports for user %s", affected, userID)
	return nil
}

// AddBucket creates a new S3 billing bucket to be monitored and processed
func (tx *Tx) AddBucket(reportID string, b *Bucket) (string, error) {
	log.Printf("Adding bucket %s under reportID: %s", b.Bucketname, reportID)
	const sqlCreateBucket = `
	INSERT INTO bucket (
		report_id,
		bucketname,
		region,
		report_path,
		aws_access_key_id,
		aws_secret_access_key
	) VALUES ($1, $2, $3, $4, $5, $6) RETURNING id;`
	var id string
	err := tx.QueryRow(sqlCreateBucket, reportID, b.Bucketname, b.Region, b.ReportPath, b.AWSAccessKeyID, b.AWSSecretAccessKey).Scan(&id)
	if err != nil {
		if strings.Contains(err.Error(), "unique_s3path") {
			return "", errors.Errorf(errors.CodeBadRequest, "Bucket '%s' with report path '%s' already configured", b.Bucketname, b.ReportPath)
		}
		return "", errors.InternalError(err)
	}
	err = tx.UpdateUserReportMtime(reportID)
	if err != nil {
		return "", err
	}
	log.Printf("Added bucket %s/%s (bucket_id: %s) to report %s", b.Bucketname, b.ReportPath, id, reportID)
	return id, nil
}

// GetBucket updates an existing bucket credentials
func (tx *Tx) GetBucket(bucketID string) (*Bucket, error) {
	var bucket Bucket
	err := tx.Get(&bucket, "SELECT * FROM bucket WHERE id = $1;", bucketID)
	if err != nil {
		return nil, errors.InternalError(err)
	}
	return &bucket, nil
}

// GetReportBuckets returns all buckets associated with a report
func (tx *Tx) GetReportBuckets(reportID string) ([]*Bucket, error) {
	buckets := []*Bucket{}
	err := tx.Select(&buckets, "SELECT * FROM bucket WHERE report_id = $1", reportID)
	if err != nil {
		return nil, errors.InternalError(err)
	}
	return buckets, nil
}

// UpdateBucket updates an existing bucket credentials
func (tx *Tx) UpdateBucket(bucket Bucket) error {
	rows, err := tx.NamedQuery("UPDATE bucket SET aws_access_key_id = :aws_access_key_id, aws_secret_access_key = :aws_secret_access_key, bucketname = :bucketname, region = :region, report_path = :report_path WHERE id = :id RETURNING report_id;", bucket)
	if err != nil {
		return errors.InternalError(err)
	}
	if rows.Next() {
		var reportID string
		err = rows.Scan(&reportID)
		if err != nil {
			return errors.InternalError(err)
		}
		rows.Close()
		err = tx.UpdateUserReportMtime(reportID)
		if err != nil {
			return err
		}
		log.Printf("Updated bucket: bucketname: %s, reportPath: %s", bucket.Bucketname, bucket.ReportPath)
	} else {
		return errors.Errorf(errors.CodeNotFound, "Bucket %s, reportPath: %s not found", bucket.Bucketname, bucket.ReportPath)
	}
	return nil
}

// DeleteBucket deletes a bucket from a report
func (tx *Tx) DeleteBucket(bucket Bucket) error {
	row := tx.QueryRow("DELETE FROM bucket WHERE bucketname = $1 AND report_path = $2 RETURNING report_id;", bucket.Bucketname, bucket.ReportPath)
	var reportID string
	err := row.Scan(&reportID)
	if err != nil {
		if err == sql.ErrNoRows {
			log.Printf("Delete of bucketname: %s, reportPath: %s ignored. Bucket does not exist", bucket.Bucketname, bucket.ReportPath)
			return nil
		}
		return errors.InternalError(err)
	}
	err = tx.UpdateUserReportMtime(reportID)
	if err != nil {
		return err
	}
	log.Printf("Deleted bucket with bucketname: %s, reportPath: %s", bucket.Bucketname, bucket.ReportPath)
	return nil
}

// AddReportAccount creates an account. This will only be called by ingestd, not by user. If account already exists, this is a noop
func (tx *Tx) AddReportAccount(reportID string, accountID string) error {
	log.Printf("Adding account %s under reportID: %s", accountID, reportID)
	_, err := tx.Exec("INSERT INTO aws_account (report_id, aws_account_id, name) VALUES ($1, $2, $3)", reportID, accountID, accountID)
	if err != nil {
		if strings.Contains(err.Error(), "unique_account") {
			log.Printf("Account %s already exists in reportID %s", accountID, reportID)
			return nil
		}
		return errors.InternalError(err)
	}
	err = tx.UpdateUserReportMtime(reportID)
	if err != nil {
		return err
	}
	log.Printf("Successfully added account %s under reportID: %s", accountID, reportID)
	return nil
}

// DeleteReportAccount deletes an AWS account associated with a report. This will only be called by ingestd, not by user
func (tx *Tx) DeleteReportAccount(reportID, accountID string) error {
	res, err := tx.Exec("DELETE FROM aws_account WHERE report_id = $1 AND aws_account_id = $2", reportID, accountID)
	if err != nil {
		return errors.InternalError(err)
	}
	affected, _ := res.RowsAffected()
	log.Printf("Deleted %d accounts with report_id = %s, aws_account_id = %s", affected, reportID, accountID)
	if affected > 0 {
		err = tx.UpdateUserReportMtime(reportID)
		if err != nil {
			return err
		}
	}
	return nil
}

// UpdateReportAcccount updates an existing bucket credentials
func (tx *Tx) UpdateReportAcccount(account *AWSAccountInfo) (*AWSAccountInfo, error) {
	log.Printf("Updating account %s as %s under reportID: %s", account.AWSAccountID, account.Name, account.ReportID)
	var updatedAccount AWSAccountInfo
	err := tx.Get(&updatedAccount, "UPDATE aws_account SET name = $1 WHERE report_id = $2 AND aws_account_id = $3 RETURNING *;", account.Name, account.ReportID, account.AWSAccountID)
	if err != nil {
		return nil, errors.InternalError(err)
	}
	err = tx.UpdateUserReportMtime(account.ReportID)
	if err != nil {
		return nil, err
	}
	log.Printf("Successfully updated account %s as %s under reportID: %s", account.AWSAccountID, account.Name, account.ReportID)
	return &updatedAccount, nil
}

// GetProduct returns the product with the given code
func (tx *Tx) GetProduct(productCode string) (*AWSProductInfo, error) {
	var productInfo AWSProductInfo
	err := tx.Get(&productInfo, "SELECT * FROM aws_product WHERE product_code = $1;", productCode)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, errors.InternalError(err)
	}
	return &productInfo, nil
}

// GetProducts returns all aws marketplace products
func (tx *Tx) GetProducts() ([]*AWSProductInfo, error) {
	products := make([]*AWSProductInfo, 0)
	rows, err := tx.Queryx("SELECT * FROM aws_product;")
	if err != nil {
		return nil, errors.InternalError(err)
	}
	for rows.Next() {
		var productInfo AWSProductInfo
		err = rows.StructScan(&productInfo)
		if err != nil {
			return nil, errors.InternalError(err)
		}
		products = append(products, &productInfo)
	}
	return products, nil
}

// UpsertProduct returns the product with the given code
func (tx *Tx) UpsertProduct(product *AWSProductInfo) error {
	_, err := tx.NamedExec("INSERT INTO aws_product (product_code, name, description) VALUES (:product_code, :name, :description) ON CONFLICT (product_code) DO UPDATE SET name = :name, description = :description;", product)
	if err != nil {
		return errors.InternalError(err)
	}
	log.Printf("Upserted product %s", *product)
	return nil
}
