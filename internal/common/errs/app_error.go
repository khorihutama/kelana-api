package errs

import (
	"errors"
	"net/http"
)

var ErrInvalidQuery = errors.New("invalid query parameter")

type FieldError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

type AppError struct {
	Code    string       `json:"code"`
	Status  int          `json:"-"`
	Message string       `json:"message"`
	Details []FieldError `json:"details,omitempty"`
	Err     error        `json:"-"`
}

func (e *AppError) Error() string {
	if e.Err != nil {
		return e.Code + ": " + e.Err.Error()
	}
	return e.Code + ": " + e.Message
}

func (e *AppError) Unwrap() error { return e.Err }

func New(code string, status int, message string) *AppError {
	return &AppError{Code: code, Status: status, Message: message}
}

func WithDetails(err *AppError, details []FieldError) *AppError {
	err.Details = details
	return err
}

func Wrap(err *AppError, cause error) *AppError {
	err.Err = cause
	return err
}

func Validation(details []FieldError) *AppError {
	return WithDetails(New(CodeValidation, http.StatusUnprocessableEntity, "Data tidak valid"), details)
}

func Unauthorized(message string) *AppError {
	return New(CodeUnauthorized, http.StatusUnauthorized, message)
}

func Forbidden(code, message string) *AppError {
	return New(code, http.StatusForbidden, message)
}

func NotFound(code, message string) *AppError {
	return New(code, http.StatusNotFound, message)
}

func Conflict(code, message string) *AppError {
	return New(code, http.StatusConflict, message)
}

func TooManyRequests(message string) *AppError {
	return New(CodeRateLimited, http.StatusTooManyRequests, message)
}

func Internal(cause error) *AppError {
	e := New(CodeInternal, http.StatusInternalServerError, "Terjadi kesalahan pada server")
	e.Err = cause
	return e
}

func As(err error) (*AppError, bool) {
	var appErr *AppError
	if errors.As(err, &appErr) {
		return appErr, true
	}
	return nil, false
}
