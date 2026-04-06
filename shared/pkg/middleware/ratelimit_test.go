package middleware

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func init() {
	gin.SetMode(gin.TestMode)
}

// newTestRedis returns a real Redis client pointing at the default local address.
// Tests that call this will be skipped if Redis is not reachable, keeping CI
// green without a running Redis sidecar.
func newTestRedis(t *testing.T) *redis.Client {
	t.Helper()
	rdb := redis.NewClient(&redis.Options{Addr: "localhost:6379"})
	t.Cleanup(func() { _ = rdb.Close() })
	return rdb
}

func redisAvailable(rdb *redis.Client) bool {
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	_, err := rdb.Ping(ctx).Result()
	return err == nil
}

// buildRateLimitRouter creates a minimal Gin engine wired with the given
// RateLimit middleware and a single GET /test handler that always returns 200.
func buildRateLimitRouter(rdb *redis.Client, cfg RateLimitConfig) *gin.Engine {
	r := gin.New()
	r.Use(RateLimit(rdb, cfg))
	r.GET("/test", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})
	return r
}

func doGet(r *gin.Engine, path string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, path, nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

// --- ChannelIDKeyFunc ---

func TestChannelIDKeyFunc_FromContextValue(t *testing.T) {
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Set("channel_id", "chan-abc")
	key := ChannelIDKeyFunc(c)
	assert.Equal(t, "channel:chan-abc", key)
}

func TestChannelIDKeyFunc_FromURLParam(t *testing.T) {
	r := gin.New()
	var got string
	r.GET("/channels/:channel_id/data", func(c *gin.Context) {
		got = ChannelIDKeyFunc(c)
		c.Status(http.StatusOK)
	})
	req := httptest.NewRequest(http.MethodGet, "/channels/chan-xyz/data", nil)
	r.ServeHTTP(httptest.NewRecorder(), req)
	assert.Equal(t, "channel:chan-xyz", got)
}

func TestChannelIDKeyFunc_FallsBackToIP(t *testing.T) {
	r := gin.New()
	var got string
	r.GET("/test", func(c *gin.Context) {
		// no channel_id set in context, no channel_id URL param
		got = ChannelIDKeyFunc(c)
		c.Status(http.StatusOK)
	})
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.RemoteAddr = "203.0.113.1:12345"
	r.ServeHTTP(httptest.NewRecorder(), req)
	assert.NotEmpty(t, got)
	assert.NotEqual(t, "channel:", got)
}

func TestChannelIDKeyFunc_EmptyContextValueFallsBackToParam(t *testing.T) {
	r := gin.New()
	var got string
	r.GET("/channels/:channel_id/data", func(c *gin.Context) {
		// context value is set but empty — should fall through to URL param
		c.Set("channel_id", "")
		got = ChannelIDKeyFunc(c)
		c.Status(http.StatusOK)
	})
	req := httptest.NewRequest(http.MethodGet, "/channels/chan-fallback/data", nil)
	r.ServeHTTP(httptest.NewRecorder(), req)
	assert.Equal(t, "channel:chan-fallback", got)
}

// --- RetryAfter header ---

func TestRateLimit_RetryAfterHeader_OnExceeded(t *testing.T) {
	rdb := newTestRedis(t)
	if !redisAvailable(rdb) {
		t.Skip("Redis not available")
	}

	uniqueKey := "test-retry-after-" + strconv.FormatInt(time.Now().UnixNano(), 10)
	cfg := RateLimitConfig{
		Requests: 2,
		Window:   time.Second,
		KeyFunc:  func(_ *gin.Context) string { return uniqueKey },
	}

	r := buildRateLimitRouter(rdb, cfg)

	// First two requests succeed.
	w1 := doGet(r, "/test")
	require.Equal(t, http.StatusOK, w1.Code)
	w2 := doGet(r, "/test")
	require.Equal(t, http.StatusOK, w2.Code)

	// Third request exceeds the limit.
	w3 := doGet(r, "/test")
	assert.Equal(t, http.StatusTooManyRequests, w3.Code)
	assert.Equal(t, "1", w3.Header().Get("Retry-After"), "Retry-After should be window in seconds")

	var body map[string]string
	require.NoError(t, json.Unmarshal(w3.Body.Bytes(), &body))
	assert.Equal(t, "rate limit exceeded", body["error"])
}

func TestRateLimit_RetryAfterHeader_LongerWindow(t *testing.T) {
	rdb := newTestRedis(t)
	if !redisAvailable(rdb) {
		t.Skip("Redis not available")
	}

	uniqueKey := "test-retry-after-60s-" + strconv.FormatInt(time.Now().UnixNano(), 10)
	cfg := RateLimitConfig{
		Requests: 1,
		Window:   time.Minute,
		KeyFunc:  func(_ *gin.Context) string { return uniqueKey },
	}

	r := buildRateLimitRouter(rdb, cfg)

	doGet(r, "/test") // consume the single slot
	w := doGet(r, "/test")

	assert.Equal(t, http.StatusTooManyRequests, w.Code)
	assert.Equal(t, "60", w.Header().Get("Retry-After"))
}

func TestRateLimit_NoRetryAfterOnSuccess(t *testing.T) {
	rdb := newTestRedis(t)
	if !redisAvailable(rdb) {
		t.Skip("Redis not available")
	}

	uniqueKey := "test-no-retry-after-" + strconv.FormatInt(time.Now().UnixNano(), 10)
	cfg := RateLimitConfig{
		Requests: 5,
		Window:   time.Second,
		KeyFunc:  func(_ *gin.Context) string { return uniqueKey },
	}

	r := buildRateLimitRouter(rdb, cfg)
	w := doGet(r, "/test")

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Empty(t, w.Header().Get("Retry-After"), "Retry-After should not be set on successful requests")
}

// --- Redis failure: fail-open ---

func TestRateLimit_RedisFailure_AllowsRequest(t *testing.T) {
	// Point at a port with nothing listening to force an immediate error.
	rdb := redis.NewClient(&redis.Options{Addr: "localhost:19999"})
	t.Cleanup(func() { _ = rdb.Close() })

	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	cfg := RateLimitConfig{
		Requests: 1,
		Window:   time.Second,
		KeyFunc:  func(_ *gin.Context) string { return "unreachable" },
		Logger:   logger,
	}

	r := buildRateLimitRouter(rdb, cfg)
	w := doGet(r, "/test")
	assert.Equal(t, http.StatusOK, w.Code, "should fail open when Redis is unavailable")
}

// --- RateLimit headers on allowed requests ---

func TestRateLimit_SetsRateLimitHeaders(t *testing.T) {
	rdb := newTestRedis(t)
	if !redisAvailable(rdb) {
		t.Skip("Redis not available")
	}

	uniqueKey := "test-headers-" + strconv.FormatInt(time.Now().UnixNano(), 10)
	cfg := RateLimitConfig{
		Requests: 5,
		Window:   time.Second,
		KeyFunc:  func(_ *gin.Context) string { return uniqueKey },
	}

	r := buildRateLimitRouter(rdb, cfg)
	w := doGet(r, "/test")

	require.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "5", w.Header().Get("X-RateLimit-Limit"))
	assert.NotEmpty(t, w.Header().Get("X-RateLimit-Remaining"))
	assert.NotEmpty(t, w.Header().Get("X-RateLimit-Reset"))
}

// --- IPKeyFunc ---

func TestIPKeyFunc_ReturnsNonEmpty(t *testing.T) {
	r := gin.New()
	var got string
	r.GET("/test", func(c *gin.Context) {
		got = IPKeyFunc(c)
		c.Status(http.StatusOK)
	})
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.RemoteAddr = "203.0.113.1:12345"
	r.ServeHTTP(httptest.NewRecorder(), req)
	assert.NotEmpty(t, got)
}

// --- UserIDKeyFunc ---

func TestUserIDKeyFunc_FromContext(t *testing.T) {
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Set(ContextKeyUserID, "user-999")
	key := UserIDKeyFunc(c)
	assert.Equal(t, "user-999", key)
}

func TestUserIDKeyFunc_FallsBackToIP(t *testing.T) {
	r := gin.New()
	var got string
	r.GET("/test", func(c *gin.Context) {
		got = UserIDKeyFunc(c)
		c.Status(http.StatusOK)
	})
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.RemoteAddr = "203.0.113.1:12345"
	r.ServeHTTP(httptest.NewRecorder(), req)
	assert.NotEmpty(t, got)
}

// --- APIKeyKeyFunc ---

func TestAPIKeyKeyFunc_FromContext(t *testing.T) {
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Set(ContextKeyAPIKey, "my-api-key")
	key := APIKeyKeyFunc(c)
	assert.Equal(t, "my-api-key", key)
}

func TestAPIKeyKeyFunc_FallsBackToIP(t *testing.T) {
	r := gin.New()
	var got string
	r.GET("/test", func(c *gin.Context) {
		got = APIKeyKeyFunc(c)
		c.Status(http.StatusOK)
	})
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.RemoteAddr = "203.0.113.1:12345"
	r.ServeHTTP(httptest.NewRecorder(), req)
	assert.NotEmpty(t, got)
}
