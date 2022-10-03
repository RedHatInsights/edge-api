// FIXME: golangci-lint
// nolint:govet,revive
package errors

import (
	"net/http"
)

// APIError defines a type for all errors returned by edge api
type APIError interface {
	Error() string
	GetStatus() int
	SetTitle(string)
}

// Error gets a error string from an APIError
func (e *apiError) Error() string { return e.Title }

func (e *apiError) GetStatus() int    { return e.Status }
func (e *apiError) SetTitle(t string) { e.Title = t }

type apiError struct {
	Code   string `json:"Code"`
	Status int    `json:"Status"`
	Title  string `json:"Title"`
}

// InternalServerError defines a generic error for Edge API
type InternalServerError struct {
	apiError
}

// NewInternalServerError creates a new InternalServerError
func NewInternalServerError() APIError {
	err := new(InternalServerError)
	err.Code = "ERROR"
	err.Title = "Something went wrong."
	err.Status = http.StatusInternalServerError
	return err
}

// BadRequest defines a error when the client's input generates an error
type BadRequest struct {
	apiError
}

// NewBadRequest creates a new BadRequest
func NewBadRequest(message string) APIError {
	err := new(BadRequest)
	err.Code = "BAD_REQUEST"
	err.Title = message
	err.Status = http.StatusBadRequest
	return err
}

// NotFound defines a error for whenever an entity is not found in the database
type NotFound struct {
	apiError
}

// NewNotFound creates a new NotFound
func NewNotFound(message string) APIError {
	err := new(NotFound)
	err.Code = "NOT_FOUND"
	err.Title = message
	err.Status = http.StatusNotFound
	return err
}

// FeatureNotAvailable defines a error when the feature is toggled off via feature flags
type FeatureNotAvailable struct {
	apiError
}

// NewFeatureNotAvailable creates a new NewFeatureNotAvailable
func NewFeatureNotAvailable(message string) APIError {
	err := new(FeatureNotAvailable)
	err.Code = "FEATURE_NOT_AVAILABLE"
	err.Title = message
	err.Status = http.StatusNotImplemented
	return err
}

// AccessForbidden  defines an error when a resource is not allowed to be served
type AccessForbidden struct {
	apiError
}

// NewAccessForbidden creates a new AccessForbidden
func NewAccessForbidden(message string) APIError {
	err := new(AccessForbidden)
	err.Code = "ACCESS_FORBIDDEN"
	err.Title = message
	err.Status = http.StatusForbidden
	return err
}
