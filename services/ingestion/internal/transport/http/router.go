package http

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/greenlab/ingestion/internal/domain"
	"github.com/greenlab/shared/pkg/apierr"
	sharedMiddleware "github.com/greenlab/shared/pkg/middleware"
	"github.com/greenlab/shared/pkg/response"
	"github.com/redis/go-redis/v9"
	ginSwagger "github.com/swaggo/gin-swagger"
	swaggerFiles "github.com/swaggo/files"
)

// APIKeyLookupFunc validates an API key using the provided request context.
type APIKeyLookupFunc func(ctx context.Context, key string) (deviceID, channelID string, err error)

func NewRouter(h *Handler, apiKeyLookup APIKeyLookupFunc, logger *slog.Logger, rdb *redis.Client) *gin.Engine {
	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(sharedMiddleware.RequestID())
	r.Use(securityHeaders())
	r.GET("/health", h.Health)
	r.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

	v1 := r.Group("/v1")
	v1.Use(apiKeyAuth(apiKeyLookup, logger))
	v1.Use(sharedMiddleware.RateLimit(rdb, sharedMiddleware.RateLimitConfig{
		Requests: 100,
		Window:   time.Minute,
		KeyFunc:  sharedMiddleware.APIKeyKeyFunc,
	}))
	{
		v1.POST("/channels/:channel_id/data", h.Ingest)
		v1.POST("/channels/:channel_id/data/bulk", h.BulkIngest)
	}
	return r
}

// apiKeyAuth validates the X-API-Key header using the request context so that
// client cancellations and deadlines propagate to Redis and Postgres.
// ErrDeviceNotFound produces 401; infrastructure errors produce 503 and are logged.
func apiKeyAuth(lookup APIKeyLookupFunc, logger *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		key := c.GetHeader("X-API-Key")
		if key == "" {
			key = c.Query("api_key")
		}
		if key == "" {
			response.Abort(c, apierr.Unauthorized("missing API key"))
			return
		}

		deviceID, channelID, err := lookup(c.Request.Context(), key)
		if err != nil {
			if errors.Is(err, domain.ErrDeviceNotFound) {
				response.Abort(c, apierr.Unauthorized("invalid API key"))
			} else {
				logger.ErrorContext(c.Request.Context(),
					"api key validation failed due to infrastructure error", "error", err)
				response.Abort(c, apierr.New(
					http.StatusServiceUnavailable,
					"service_unavailable",
					"authentication service temporarily unavailable",
				))
			}
			return
		}

		c.Set("device_id", deviceID)
		c.Set("channel_id", channelID)
		c.Set(sharedMiddleware.ContextKeyAPIKey, key)
		c.Next()
	}
}

func securityHeaders() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("X-Content-Type-Options", "nosniff")
		c.Header("X-Frame-Options", "DENY")
		c.Header("Referrer-Policy", "strict-origin-when-cross-origin")
		c.Next()
	}
}
