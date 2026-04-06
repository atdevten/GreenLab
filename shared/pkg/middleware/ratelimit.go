package middleware

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
)

// RateLimitConfig defines settings for the rate limiter.
type RateLimitConfig struct {
	// Requests is the maximum number of requests allowed per Window.
	Requests int
	// Window is the duration of the sliding window.
	Window time.Duration
	// KeyFunc extracts a key from the request (e.g., IP or user ID).
	KeyFunc func(c *gin.Context) string
	// Logger is an optional structured logger. When non-nil, Redis failures are
	// logged at Error level. A nil Logger means silent fail-open behaviour.
	Logger *slog.Logger
}

// RateLimit returns a Redis-backed sliding window rate limiter middleware.
func RateLimit(rdb *redis.Client, cfg RateLimitConfig) gin.HandlerFunc {
	return func(c *gin.Context) {
		key := cfg.KeyFunc(c)
		redisKey := fmt.Sprintf("ratelimit:%s", key)

		ctx := context.Background()
		now := time.Now()
		windowStart := now.Add(-cfg.Window)

		pipe := rdb.Pipeline()
		// Remove expired entries
		pipe.ZRemRangeByScore(ctx, redisKey, "0", strconv.FormatInt(windowStart.UnixNano(), 10))
		// Count remaining
		countCmd := pipe.ZCard(ctx, redisKey)
		// Add current request
		pipe.ZAdd(ctx, redisKey, redis.Z{
			Score:  float64(now.UnixNano()),
			Member: now.UnixNano(),
		})
		// Set expiry
		pipe.Expire(ctx, redisKey, cfg.Window)

		_, err := pipe.Exec(ctx)
		if err != nil {
			// On Redis failure, allow the request
			if cfg.Logger != nil {
				cfg.Logger.ErrorContext(c.Request.Context(), "rate limiter redis failure — allowing request", "error", err, "key", key)
			}
			c.Next()
			return
		}

		count := countCmd.Val()
		remaining := int64(cfg.Requests) - count - 1
		if remaining < 0 {
			remaining = 0
		}

		c.Header("X-RateLimit-Limit", strconv.Itoa(cfg.Requests))
		c.Header("X-RateLimit-Remaining", strconv.FormatInt(remaining, 10))
		c.Header("X-RateLimit-Reset", strconv.FormatInt(now.Add(cfg.Window).Unix(), 10))

		if count >= int64(cfg.Requests) {
			retryAfter := int(cfg.Window.Seconds())
			if retryAfter < 1 {
				retryAfter = 1
			}
			c.Header("Retry-After", strconv.Itoa(retryAfter))
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
				"error": "rate limit exceeded",
			})
			return
		}

		c.Next()
	}
}

// IPKeyFunc extracts the client IP as the rate limit key.
func IPKeyFunc(c *gin.Context) string {
	return c.ClientIP()
}

// UserIDKeyFunc extracts the authenticated user ID as the rate limit key.
func UserIDKeyFunc(c *gin.Context) string {
	if uid, ok := c.Get(ContextKeyUserID); ok {
		if s, ok := uid.(string); ok && s != "" {
			return s
		}
	}
	return c.ClientIP()
}

// APIKeyKeyFunc extracts the API key as the rate limit key.
func APIKeyKeyFunc(c *gin.Context) string {
	if key, ok := c.Get(ContextKeyAPIKey); ok {
		if s, ok := key.(string); ok && s != "" {
			return s
		}
	}
	return c.ClientIP()
}

// ChannelIDKeyFunc extracts the channel_id from context as the rate limit key.
// Falls back to the URL param "channel_id" if the context value is absent, and
// finally to the client IP so the middleware always returns a non-empty key.
func ChannelIDKeyFunc(c *gin.Context) string {
	if v, ok := c.Get("channel_id"); ok {
		if s, ok := v.(string); ok && s != "" {
			return "channel:" + s
		}
	}
	if id := c.Param("channel_id"); id != "" {
		return "channel:" + id
	}
	return c.ClientIP()
}
