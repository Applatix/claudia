// Copyright 2017 Applatix, Inc.
package errors

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/pkg/errors"
)

// Externally visible error codes
var (
	CodeUnauthorized = "ERR_UNAUTHORIZED"
	CodeBadRequest   = "ERR_BAD_REQUEST"
	CodeForbidden    = "ERR_FORBIDDEN"
	CodeNotFound     = "ERR_NOT_FOUND"
	CodeInternal     = "ERR_INTERNAL"
)

// APIError is an error interface that additionally adds support for stack trace, error code, and http status code
type APIError interface {
	Error() string
	Code() string
	JSON() []byte
	HTTPStatusCode() int
	StackTrace() errors.StackTrace
}

// New returns an error with the supplied message.
// New also records the stack trace at the point it was called.
func New(code string, message string) error {
	err := errors.New(message)
	return apierror{err, code, err.(pkgError)}
}

// Errorf formats according to a format specifier and returns the string
// as a value that satisfies error.
// Errorf also records the stack trace at the point it was called.
func Errorf(code string, format string, args ...interface{}) error {
	return New(code, fmt.Sprintf(format, args...))
}

// WithStack annotates err with a stack trace at the point WithStack was called.
// If err is nil, WithStack returns nil.
func WithStack(err error, code string) error {
	if err == nil {
		return nil
	}
	err = errors.WithStack(err)
	return apierror{err, code, err.(pkgError)}
}

// Wrap returns an error annotating err with a stack trace at the point Wrap is called,
// and a new supplied message. The previous original is preserved and accessible via Cause().
// If err is nil, Wrap returns nil.
func Wrap(err error, code string, message string) error {
	if err == nil {
		return nil
	}
	err = errors.Wrap(err, message)
	return apierror{err, code, err.(pkgError)}
}

// InternalError annotates the error with the internal error code and a stack trace
func InternalError(err error) error {
	return WithStack(err, CodeInternal)
}

// InternalErrorWithMessage annotates the error with the internal error code and a stack trace and message
func InternalErrorWithMessage(err error, message string) error {
	return Wrap(err, CodeInternal, message)
}

// InternalErrorf annotates the error with the internal error code and a stack trace and a formatted message
func InternalErrorf(err error, format string, args ...interface{}) error {
	return Wrap(err, CodeInternal, fmt.Sprintf(format, args...))
}

// apiError embeds a pkg error and additionally implements Code and JSON marshaller, HTTPStatusCode methods
type apierror struct {
	error
	code   string
	pkgerr pkgError
}

func (e apierror) Error() string {
	return fmt.Sprintf("[%s] %s", e.code, e.pkgerr.Error())
}

func (e apierror) Code() string {
	return e.code
}

func (e apierror) JSON() []byte {
	type errBean struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	}
	eb := errBean{e.code, e.pkgerr.Error()}
	j, _ := json.Marshal(eb)
	return j
}

func (e apierror) HTTPStatusCode() int {
	switch e.code {
	case CodeNotFound:
		return http.StatusNotFound
	case CodeForbidden:
		return http.StatusForbidden
	case CodeBadRequest:
		return http.StatusBadRequest
	case CodeUnauthorized:
		return http.StatusUnauthorized
	case CodeInternal:
		return http.StatusInternalServerError
	default:
		return http.StatusInternalServerError
	}
}

type pkgError interface {
	Error() string
	StackTrace() errors.StackTrace
	Format(s fmt.State, verb rune)
}

func (e apierror) StackTrace() errors.StackTrace {
	return e.pkgerr.StackTrace()
}

func (e apierror) Format(s fmt.State, verb rune) {
	switch verb {
	case 'v':
		if s.Flag('+') {
			io.WriteString(s, e.Error())
			for _, pc := range e.pkgerr.StackTrace() {
				f := errors.Frame(pc)
				fmt.Fprintf(s, "\n%+v", f)
			}
			return
		}
		fallthrough
	case 's':
		io.WriteString(s, e.Error())
	case 'q':
		fmt.Fprintf(s, "%q", e.Error())
	}
}

// Cause returns the underlying cause of the error, if possible.
func Cause(err error) error {
	if apiErr, ok := err.(apierror); ok {
		return errors.Cause(apiErr.pkgerr)
	}
	return errors.Cause(err)
}
