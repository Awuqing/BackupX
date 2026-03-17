package apperror

import "net/http"

type AppError struct {
	Status  int
	Code    string
	Message string
	Err     error
}

func (e *AppError) Error() string {
	if e == nil {
		return ""
	}
	if e.Err != nil {
		return e.Err.Error()
	}
	return e.Message
}

func (e *AppError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

func New(status int, code, message string, err error) *AppError {
	return &AppError{Status: status, Code: code, Message: message, Err: err}
}

func BadRequest(code, message string, err error) *AppError {
	return New(http.StatusBadRequest, code, message, err)
}

func Unauthorized(code, message string, err error) *AppError {
	return New(http.StatusUnauthorized, code, message, err)
}

func Forbidden(code, message string, err error) *AppError {
	return New(http.StatusForbidden, code, message, err)
}

func Conflict(code, message string, err error) *AppError {
	return New(http.StatusConflict, code, message, err)
}

func TooManyRequests(code, message string, err error) *AppError {
	return New(http.StatusTooManyRequests, code, message, err)
}

func Internal(code, message string, err error) *AppError {
	return New(http.StatusInternalServerError, code, message, err)
}
