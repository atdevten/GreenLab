package response

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/greenlab/shared/pkg/apierr"
)

// Envelope is the standard API response body.
type Envelope struct {
	Success bool        `json:"success"`
	Data    interface{} `json:"data,omitempty"`
	Error   *ErrBody    `json:"error,omitempty"`
	Meta    interface{} `json:"meta,omitempty"`
}

// ErrBody represents the error portion of an API response.
type ErrBody struct {
	Code    string      `json:"code"`
	Message string      `json:"message"`
	Details interface{} `json:"details,omitempty"`
}

// OK sends a 200 response with data.
func OK(c *gin.Context, data interface{}) {
	c.JSON(http.StatusOK, Envelope{Success: true, Data: data})
}

// OKWithMeta sends a 200 response with data and meta.
func OKWithMeta(c *gin.Context, data, meta interface{}) {
	c.JSON(http.StatusOK, Envelope{Success: true, Data: data, Meta: meta})
}

// Created sends a 201 response with data.
func Created(c *gin.Context, data interface{}) {
	c.JSON(http.StatusCreated, Envelope{Success: true, Data: data})
}

// NoContent sends a 204 response.
func NoContent(c *gin.Context) {
	c.Status(http.StatusNoContent)
}

// Error sends an error response based on the error type.
func Error(c *gin.Context, err error) {
	var apiErr *apierr.APIError
	if errors.As(err, &apiErr) {
		c.JSON(apiErr.Code, Envelope{
			Success: false,
			Error: &ErrBody{
				Code:    apiErr.Type,
				Message: apiErr.Detail,
			},
		})
		return
	}

	// Unrecognized error → 500
	c.JSON(http.StatusInternalServerError, Envelope{
		Success: false,
		Error: &ErrBody{
			Code:    "internal_server_error",
			Message: "an unexpected error occurred",
		},
	})
}

// ValidationError sends a 422 response with validation error details.
func ValidationError(c *gin.Context, details interface{}) {
	c.JSON(http.StatusUnprocessableEntity, Envelope{
		Success: false,
		Error: &ErrBody{
			Code:    "validation_error",
			Message: "request validation failed",
			Details: details,
		},
	})
}

// Abort sends an error response and aborts the middleware chain.
func Abort(c *gin.Context, err error) {
	var apiErr *apierr.APIError
	if errors.As(err, &apiErr) {
		c.AbortWithStatusJSON(apiErr.Code, Envelope{
			Success: false,
			Error: &ErrBody{
				Code:    apiErr.Type,
				Message: apiErr.Detail,
			},
		})
		return
	}

	c.AbortWithStatusJSON(http.StatusInternalServerError, Envelope{
		Success: false,
		Error: &ErrBody{
			Code:    "internal_server_error",
			Message: "an unexpected error occurred",
		},
	})
}
