package apierr

import (
	"errors"
	"fmt"
	"net/http"
)

// APIError represents a structured API error with HTTP status code.
type APIError struct {
	Code   int    `json:"code"`
	Type   string `json:"type"`             // machine-readable category e.g. "bad_request"
	Detail string `json:"detail,omitempty"` // human-readable description
}

func (e *APIError) Error() string {
	return fmt.Sprintf("[%d] %s: %s", e.Code, e.Type, e.Detail)
}

// New creates a new APIError.
func New(code int, errType, detail string) *APIError {
	return &APIError{Code: code, Type: errType, Detail: detail}
}

// Wrap wraps an existing error into an APIError.
func Wrap(code int, errType string, err error) *APIError {
	detail := ""
	if err != nil {
		detail = err.Error()
	}
	return &APIError{Code: code, Type: errType, Detail: detail}
}

// Is implements errors.Is interface.
func (e *APIError) Is(target error) bool {
	var t *APIError
	if errors.As(target, &t) {
		return t.Code == e.Code
	}
	return false
}

// Predefined errors.
var (
	ErrBadRequest          = New(http.StatusBadRequest, "bad_request", "")
	ErrUnauthorized        = New(http.StatusUnauthorized, "unauthorized", "")
	ErrForbidden           = New(http.StatusForbidden, "forbidden", "")
	ErrNotFound            = New(http.StatusNotFound, "not_found", "")
	ErrConflict            = New(http.StatusConflict, "conflict", "")
	ErrUnprocessable       = New(http.StatusUnprocessableEntity, "unprocessable_entity", "")
	ErrTooManyRequests     = New(http.StatusTooManyRequests, "too_many_requests", "")
	ErrInternalServerError = New(http.StatusInternalServerError, "internal_server_error", "an unexpected error occurred")
)

// NotFound returns a 404 APIError with a detail message.
func NotFound(resource string) *APIError {
	return New(http.StatusNotFound, "not_found", fmt.Sprintf("%s not found", resource))
}

// BadRequest returns a 400 APIError.
func BadRequest(detail string) *APIError {
	return New(http.StatusBadRequest, "bad_request", detail)
}

// Unauthorized returns a 401 APIError.
func Unauthorized(detail string) *APIError {
	return New(http.StatusUnauthorized, "unauthorized", detail)
}

// Forbidden returns a 403 APIError.
func Forbidden(detail string) *APIError {
	return New(http.StatusForbidden, "forbidden", detail)
}

// Conflict returns a 409 APIError.
func Conflict(detail string) *APIError {
	return New(http.StatusConflict, "conflict", detail)
}

// Internal returns a 500 APIError wrapping err. The detail is included for internal
// logging; callers that send this to clients should use ErrInternalServerError instead.
func Internal(err error) *APIError {
	return Wrap(http.StatusInternalServerError, "internal_server_error", err)
}

// HTTPCode extracts HTTP status code from an error.
// Returns 500 if the error is not an APIError.
func HTTPCode(err error) int {
	var apiErr *APIError
	if errors.As(err, &apiErr) {
		return apiErr.Code
	}
	return http.StatusInternalServerError
}
