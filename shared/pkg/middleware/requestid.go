package middleware

import (
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

const (
	// RequestIDHeader is the HTTP header name for the request ID.
	RequestIDHeader = "X-Request-ID"
	// ContextKeyRequestID is the context key for the request ID.
	ContextKeyRequestID = "request_id"
)

// RequestID is a Gin middleware that assigns a unique request ID to every request.
// It reads from the X-Request-ID header if present, otherwise generates a new UUID.
func RequestID() gin.HandlerFunc {
	return func(c *gin.Context) {
		requestID := c.GetHeader(RequestIDHeader)
		if requestID == "" {
			requestID = uuid.New().String()
		}

		c.Set(ContextKeyRequestID, requestID)
		c.Header(RequestIDHeader, requestID)
		c.Next()
	}
}

// GetRequestID retrieves the request ID from the Gin context.
func GetRequestID(c *gin.Context) string {
	if v, exists := c.Get(ContextKeyRequestID); exists {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}
