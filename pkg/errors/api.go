package errors

import (
	"net/http"
)

type APIError struct {
	Code   string
	Status int
	Title  string
}

func (e *APIError) Error() string { return e.Title }

type InternalServerError struct {
	APIError
}

func NewInternalServerError() *InternalServerError {
	err := new(InternalServerError)
	err.Code = "ERROR"
	err.Title = "Something wrong happened."
	err.Status = http.StatusInternalServerError
	return err
}

type BadRequest struct {
	APIError
}

func NewBadRequest(message string) *BadRequest {
	err := new(BadRequest)
	err.Code = "BAD_REQUEST"
	err.Title = message
	err.Status = http.StatusBadRequest
	return err
}

type NotFound struct {
	APIError
}

func NewNotFound(message string) *NotFound {
	err := new(NotFound)
	err.Code = "NOT_FOUND"
	err.Title = message
	err.Status = http.StatusNotFound
	return err
}
