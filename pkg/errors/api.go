package errors

import (
	"net/http"
)

type APIError struct {
	Code   string
	Status int
	Title  string
}

type InternalServerError struct {
	APIError
}

func (e *InternalServerError) Error() string { return "Something wrong happened." }

func NewInternalServerError() *InternalServerError {
	err := new(InternalServerError)
	err.Code = "ERROR"
	err.Title = err.Error()
	err.Status = http.StatusInternalServerError
	return err
}
