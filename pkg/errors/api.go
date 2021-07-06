package errors

import (
	"net/http"
)

// Edge API specific error type with context
type APIError struct {
	Code   string `json:"Code"`
	Status int    `json:"Status"`
	Title  string `json:"Title"`
}

func (e *APIError) Error() string { return e.Title }

// APIError type to specifically handle internal server errors
type InternalServerError struct {
	APIError
}

// Create anew InternalServerError struct
func NewInternalServerError() *InternalServerError {
	err := new(InternalServerError)
	err.Code = "ERROR"
	err.Title = "Something wrong happened."
	err.Status = http.StatusInternalServerError
	return err
}

// APIError type to specifically handle bad requests
type BadRequest struct {
	APIError
}

// Create a new BadQuest error struct
func NewBadRequest(message string) *BadRequest {
	err := new(BadRequest)
	err.Code = "BAD_REQUEST"
	err.Title = message
	err.Status = http.StatusBadRequest
	return err
}

// APIError type to specifically handle 404s
type NotFound struct {
	APIError
}

// Create a new NotFound error struct
func NewNotFound(message string) *NotFound {
	err := new(NotFound)
	err.Code = "NOT_FOUND"
	err.Title = message
	err.Status = http.StatusNotFound
	return err
}
