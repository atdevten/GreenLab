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

// APIKeyLookupFunc validates an API key + channelID pair and returns the device schema.
type APIKeyLookupFunc func(ctx context.Context, key, channelID string) (domain.DeviceSchema, error)

// ChannelLookupFunc resolves a channel by API key alone (ThingSpeak-style).
type ChannelLookupFunc func(ctx context.Context, apiKey string) (domain.DeviceSchema, error)

func NewRouter(h *Handler, apiKeyLookup APIKeyLookupFunc, channelLookup ChannelLookupFunc, logger *slog.Logger, rdb *redis.Client) *gin.Engine {
	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(sharedMiddleware.RequestID())
	r.Use(securityHeaders())
	r.GET("/health", h.Health)
	r.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))
	tsHandler := h.ThingSpeak(channelLookup, logger, rdb)
	r.GET("/update", tsHandler)
	r.POST("/update", tsHandler)

	v1 := r.Group("/v1")
	v1.Use(apiKeyAuth(apiKeyLookup, logger))
	v1.Use(sharedMiddleware.RateLimit(rdb, sharedMiddleware.RateLimitConfig{
		Requests: 100,
		Window:   time.Minute,
		KeyFunc:  sharedMiddleware.APIKeyKeyFunc,
		Logger:   logger,
	}))
	// Per-channel rate limit scoped only to channel data routes to avoid applying
	// IP-based fallback limiting to future v1 routes without a channel_id.
	channelData := v1.Group("/channels/:channel_id")
	channelData.Use(sharedMiddleware.RateLimit(rdb, sharedMiddleware.RateLimitConfig{
		Requests: 10,
		Window:   time.Second,
		KeyFunc:  sharedMiddleware.ChannelIDKeyFunc,
		Logger:   logger,
	}))
	{
		channelData.POST("/data", h.Ingest)
		channelData.POST("/data/bulk", h.BulkIngest)
	}
	return r
}

// apiKeyAuth validates the X-API-Key header using the request context so that
// client cancellations and deadlines propagate to Redis and device-registry.
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

		channelID := c.Param("channel_id")
		schema, err := lookup(c.Request.Context(), key, channelID)
		if err != nil {
			if errors.Is(err, domain.ErrDeviceNotFound) {
				response.Abort(c, apierr.Unauthorized("invalid API key or channel"))
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

		// Store full schema for compact-format handlers, plus device_id for backward compat.
		c.Set("device_schema", schema)
		c.Set("device_id", schema.DeviceID)
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
