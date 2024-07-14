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

// NoContent defines an error where the file was not found, but it's necessary to return a 204
type NoContent struct {
	apiError
}

// NewNoContent creat a new  NoContent error
func NewNoContent(message string) APIError {
	err := new(NoContent)
	err.Code = "NO_CONTENT"
	err.Title = message
	err.Status = http.StatusNoContent

	return err
}

// FeatureNotAvailable defines a error when the feature is toggled off via feature flags
type FeatureNotAvailable struct {
	apiError
}

// FeatureNotAvailableDefaultMessage the message used by default for FeatureNotAvailable API error
const FeatureNotAvailableDefaultMessage = "Feature not available"

// NewFeatureNotAvailable creates a new NewFeatureNotAvailable
func NewFeatureNotAvailable(message string) APIError {
	if message == "" {
		message = FeatureNotAvailableDefaultMessage
	}
	err := new(FeatureNotAvailable)
	err.Code = "FEATURE_NOT_AVAILABLE"
	err.Title = message
	err.Status = http.StatusNotImplemented
	return err
}

// Forbidden defines an error for whenever access is forbidden
type Forbidden struct {
	apiError
}

// ForbiddenDefaultMessage the message used by default for Forbidden API error
const ForbiddenDefaultMessage = "access is forbidden"

// NewForbidden creates a new Forbidden API error
func NewForbidden(message string) APIError {
	if message == "" {
		message = ForbiddenDefaultMessage
	}
	err := new(FeatureNotAvailable)
	err.Code = "FORBIDDEN"
	err.Title = message
	err.Status = http.StatusForbidden
	return err
}

// ServiceUnavailable defines an error for whenever service is unavailable
type ServiceUnavailable struct {
	apiError
}

// ServiceUnavailableDefaultMessage the message used by default for Service Unavailable API error
const ServiceUnavailableDefaultMessage = "service is unavailable"

// NewServiceUnavailable creates a new ServiceUnavailable API error
func NewServiceUnavailable(message string) APIError {
	if message == "" {
		message = ServiceUnavailableDefaultMessage
	}
	err := new(FeatureNotAvailable)
	err.Code = "SERVICE_UNAVAILABLE"
	err.Title = message
	err.Status = http.StatusServiceUnavailable
	return err
}
