package apperr

import (
	"errors"
	"net/http"
)

type AppError struct {
	Code       string `json:"code"`
	Message    string `json:"message"`
	StatusCode int    `json:"-"`
	Err        error  `json:"-"`
}

func (e *AppError) Error() string { return e.Message }
func (e *AppError) Unwrap() error { return e.Err }

func New(code string, status int, msg string) *AppError {
	return &AppError{Code: code, StatusCode: status, Message: msg}
}

func Wrap(err error, code string, status int, msg string) *AppError {
	return &AppError{Code: code, StatusCode: status, Message: msg, Err: err}
}

func ToHTTP(err error) *AppError {
	var ae *AppError
	if errors.As(err, &ae) {
		return ae
	}
	return Wrap(err, "INTERNAL", http.StatusInternalServerError, "internal server error")
}
