package errors

import (
	"net/http"
)

// APIError defines a type for all errors returned by edge api
type APIError struct {
	Code   string `json:"Code"`
	Status int    `json:"Status"`
	Title  string `json:"Title"`
}

// Error gets a error string from an APIError
func (e *APIError) Error() string { return e.Title }

// InternalServerError defines a generic error for Edge API
type InternalServerError struct {
	APIError
}

// NewInternalServerError creates a new InternalServerError
func NewInternalServerError() *InternalServerError {
	err := new(InternalServerError)
	err.Code = "ERROR"
	err.Title = "Something went wrong."
	err.Status = http.StatusInternalServerError
	return err
}

// BadRequest defines a error when the client's input generates an error
type BadRequest struct {
	APIError
}

// NewBadRequest creates a new BadRequest
func NewBadRequest(message string) *BadRequest {
	err := new(BadRequest)
	err.Code = "BAD_REQUEST"
	err.Title = message
	err.Status = http.StatusBadRequest
	return err
}

// NotFound defines a error for whenever an entity is not found in the database
type NotFound struct {
	APIError
}

// NewNotFound creates a new NotFound
func NewNotFound(message string) *NotFound {
	err := new(NotFound)
	err.Code = "NOT_FOUND"
	err.Title = message
	err.Status = http.StatusNotFound
	return err
}
