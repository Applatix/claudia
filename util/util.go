// Copyright 2017 Applatix, Inc.
package util

import (
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"log"
	"math/big"
	"net/http"
	"os"
	"os/signal"
	"reflect"
	"regexp"
	"runtime"
	"syscall"
	"time"

	"github.com/applatix/claudia/errors"
)

// SuccessHandler is a helper to write an json response success APIs.
// If obj is nil, returns empty dictionary.
// If obj is an array or slice, encapsulates it into a json list in a "data" field (e.g. {"data" : []})
// For all other cases (structs/maps), returns the object marshaled as json
func SuccessHandler(obj interface{}, w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	if obj == nil {
		w.Write([]byte("{}"))
		return
	}
	objKind := reflect.TypeOf(obj).Kind()
	if objKind == reflect.Slice || objKind == reflect.Array {
		type apiResultCollection struct {
			// Data is an array/slice of elements
			Data interface{} `json:"data"`
		}
		var result = new(apiResultCollection)
		result.Data = obj
		j, err := json.Marshal(result)
		if ErrorHandler(err, w) != nil {
			return
		}
		w.Write(j)
	} else {
		j, err := json.Marshal(obj)
		if ErrorHandler(err, w) != nil {
			return
		}
		w.Write(j)
	}
}

// TXErrorHandler is a helper to rollback a transaction and write a json error to the response
func TXErrorHandler(err error, tx interface{}, w http.ResponseWriter) (origerr error) {
	origerr = err
	defer func() {
		if r := recover(); r != nil {
			log.Println("Rollback failed", r)
		}
		return
	}()
	if err != nil {
		ErrorHandler(err, w)
		if tx != nil {
			type txRollback interface {
				Rollback() error
			}
			if txrb, ok := tx.(txRollback); ok {
				txrb.Rollback()
			}
		}
	}
	return
}

// ErrorHandler is a helper log and write a json error bean to the response
func ErrorHandler(err error, w http.ResponseWriter) error {
	if err != nil {
		var apiErr errors.APIError
		var ok bool
		if apiErr, ok = err.(errors.APIError); ok {
			if apiErr.HTTPStatusCode() >= http.StatusInternalServerError {
				log.Printf("%+v", apiErr)
			} else {
				log.Println(apiErr)
			}
		} else {
			apiErr = errors.New(errors.CodeInternal, err.Error()).(errors.APIError)
			log.Println(apiErr)
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.WriteHeader(apiErr.HTTPStatusCode())
		w.Write(apiErr.JSON())
	}
	return err
}

// StartStatsTicker will start a goroutine which dumps stats at a specified interval
func StartStatsTicker(d time.Duration) {
	ticker := time.NewTicker(d)
	go func() {
		for {
			<-ticker.C
			LogStats()
		}
	}()
}

// RegisterStackDumper will start a goroutine which waits to receive a SIGUSR1 signal and upon receival, dumps stack trace
func RegisterStackDumper() {
	go func() {
		sigs := make(chan os.Signal, 1)
		signal.Notify(sigs, syscall.SIGUSR1)
		for {
			<-sigs
			log.Println("=== received SIGUSR1 ===")
			LogStack()
			LogStats()
		}
	}()
}

// LogStack will log the current stack
func LogStack() {
	buf := make([]byte, 1<<20)
	stacklen := runtime.Stack(buf, true)
	log.Printf("*** goroutine dump...\n%s\n*** end\n", buf[:stacklen])
}

// LogStats will log the current memory usage and number of goroutines
func LogStats() {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	log.Printf("Alloc=%v TotalAlloc=%v Sys=%v NumGC=%v Goroutines=%d", m.Alloc/1024, m.TotalAlloc/1024, m.Sys/1024, m.NumGC, runtime.NumGoroutine())
}

// GenerateSelfSignedCert generates self signed certificate and the signing key
func GenerateSelfSignedCert() (crt, key string) {
	// ok, lets populate the certificate with some data
	// not all fields in Certificate will be populated
	// see Certificate structure at
	// http://golang.org/pkg/crypto/x509/#Certificate
	template := &x509.Certificate{
		IsCA: true,
		BasicConstraintsValid: true,
		SubjectKeyId:          []byte{1, 2, 3},
		SerialNumber:          big.NewInt(1234),
		Subject: pkix.Name{
			Country:      []string{"United States"},
			Organization: []string{"Applatix Inc."},
		},
		NotBefore: time.Now(),
		// Make the certificate to be valid for the following 4 years
		NotAfter: time.Now().AddDate(4, 0, 0),
		// see http://golang.org/pkg/crypto/x509/#KeyUsage
		ExtKeyUsage:        []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageServerAuth},
		KeyUsage:           x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		SignatureAlgorithm: x509.SHA512WithRSA,
	}

	// generate private key
	privatekey, err := rsa.GenerateKey(rand.Reader, 2048)

	if err != nil {
		fmt.Println(err)
	}

	publickey := &privatekey.PublicKey

	cert, err := x509.CreateCertificate(rand.Reader, template, template, publickey, privatekey)

	if err != nil {
		fmt.Println(err)
	}

	keyB := &bytes.Buffer{}
	var pemkey = &pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(privatekey)}
	pem.Encode(keyB, pemkey)
	key = keyB.String()

	crtB := &bytes.Buffer{}
	var pemCrt = &pem.Block{
		Type:  "CERTIFICATE",
		Bytes: cert}
	pem.Encode(crtB, pemCrt)
	crt = crtB.String()

	return
}

var uuidv4Matcher = regexp.MustCompile("^[a-fA-F0-9]{8}-[a-fA-F0-9]{4}-4[a-fA-F0-9]{3}-[8|9|aA|bB][a-fA-F0-9]{3}-[a-fA-F0-9]{12}$")

// IsUUIDv4 returns whether or not the string is a UUIDv4
func IsUUIDv4(u string) bool {
	return uuidv4Matcher.MatchString(u)
}
