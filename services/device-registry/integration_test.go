//go:build integration

package integration_test

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
	tcredis "github.com/testcontainers/testcontainers-go/modules/redis"

	"github.com/greenlab/device-registry/internal/application"
	"github.com/greenlab/device-registry/internal/domain/device"
	"github.com/greenlab/device-registry/internal/infrastructure/postgres"
	deviceredis "github.com/greenlab/device-registry/internal/infrastructure/redis"
	transporthttp "github.com/greenlab/device-registry/internal/transport/http"
	sharedMiddleware "github.com/greenlab/shared/pkg/middleware"
)

// testSchema is the minimal DDL needed to support the device-registry integration tests.
const testSchema = `
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

CREATE TABLE IF NOT EXISTS devices (
  id           UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
  workspace_id UUID NOT NULL,
  name         TEXT NOT NULL,
  description  TEXT NOT NULL DEFAULT '',
  api_key      TEXT NOT NULL UNIQUE DEFAULT '',
  status       TEXT NOT NULL DEFAULT 'active',
  last_seen_at TIMESTAMPTZ,
  metadata     JSONB,
  created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  deleted_at   TIMESTAMPTZ
);

CREATE TABLE IF NOT EXISTS channels (
  id           UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
  workspace_id UUID NOT NULL,
  device_id    UUID REFERENCES devices(id) ON DELETE SET NULL,
  name         TEXT NOT NULL,
  description  TEXT NOT NULL DEFAULT '',
  visibility   TEXT NOT NULL DEFAULT 'private',
  tags         JSONB NOT NULL DEFAULT '[]',
  created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  deleted_at   TIMESTAMPTZ
);
`

// startPostgres spins up a Postgres testcontainer, applies the schema, and returns
// a connected *sqlx.DB. The container is terminated via t.Cleanup.
func startPostgres(t *testing.T) *sqlx.DB {
	t.Helper()
	ctx := context.Background()

	ctr, err := tcpostgres.Run(ctx,
		"postgres:16-alpine",
		tcpostgres.WithDatabase("testdb"),
		tcpostgres.WithUsername("testuser"),
		tcpostgres.WithPassword("testpass"),
		tcpostgres.BasicWaitStrategies(),
	)
	require.NoError(t, err, "start postgres container")
	t.Cleanup(func() { _ = ctr.Terminate(ctx) })

	dsn, err := ctr.ConnectionString(ctx, "sslmode=disable")
	require.NoError(t, err, "get postgres connection string")

	db, err := sqlx.Open("postgres", dsn)
	require.NoError(t, err, "open sqlx connection")
	t.Cleanup(func() { db.Close() })

	// Wait until Postgres is ready.
	require.Eventually(t, func() bool {
		return db.PingContext(ctx) == nil
	}, 30*time.Second, 200*time.Millisecond, "postgres did not become ready")

	db.MustExec(testSchema)
	return db
}

// startRedis spins up a Redis testcontainer and returns a connected *redis.Client.
// The container is terminated via t.Cleanup.
func startRedis(t *testing.T) *redis.Client {
	t.Helper()
	ctx := context.Background()

	ctr, err := tcredis.Run(ctx, "redis:7-alpine")
	require.NoError(t, err, "start redis container")
	t.Cleanup(func() { _ = ctr.Terminate(ctx) })

	endpoint, err := ctr.Endpoint(ctx, "")
	require.NoError(t, err, "get redis endpoint")

	client := redis.NewClient(&redis.Options{Addr: endpoint})
	t.Cleanup(func() { client.Close() })

	require.Eventually(t, func() bool {
		return client.Ping(ctx).Err() == nil
	}, 30*time.Second, 200*time.Millisecond, "redis did not become ready")

	return client
}

// tenantMiddleware injects the given workspace ID as the tenant claim so handlers
// can call sharedMiddleware.GetTenantID without a real JWT.
func tenantMiddleware(workspaceID string) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Set(sharedMiddleware.ContextKeyTenantID, workspaceID)
		c.Next()
	}
}

// insertDevice inserts a device row directly into the DB and returns the assigned UUID.
func insertDevice(t *testing.T, db *sqlx.DB, workspaceID uuid.UUID, name, apiKey string) uuid.UUID {
	t.Helper()
	id := uuid.New()
	_, err := db.ExecContext(context.Background(), `
		INSERT INTO devices (id, workspace_id, name, api_key, status)
		VALUES ($1, $2, $3, $4, 'active')`,
		id, workspaceID, name, apiKey,
	)
	require.NoError(t, err, "insert device %s", name)
	return id
}

// insertChannel inserts a channel row directly into the DB and returns its UUID.
func insertChannel(t *testing.T, db *sqlx.DB, workspaceID uuid.UUID, name string) uuid.UUID {
	t.Helper()
	id := uuid.New()
	_, err := db.ExecContext(context.Background(), `
		INSERT INTO channels (id, workspace_id, name, visibility, tags)
		VALUES ($1, $2, $3, 'private', '[]')`,
		id, workspaceID, name,
	)
	require.NoError(t, err, "insert channel %s", name)
	return id
}

// newTestRouter wires up a minimal Gin router with the given tenant ID injected,
// routing GET /devices/:id and GET /channels/:id.
func newTestRouter(
	wsID string,
	deviceHandler *transporthttp.DeviceHandler,
	channelHandler *transporthttp.ChannelHandler,
) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(tenantMiddleware(wsID))
	r.GET("/devices/:id", deviceHandler.GetDevice)
	r.GET("/channels/:id", channelHandler.GetChannel)
	r.POST("/devices/:id/rotate-key", deviceHandler.RotateAPIKey)
	return r
}

// ─── Tests ──────────────────────────────────────────────────────────────────

// TestCrossTenant_DeviceA_CannotReadDeviceB verifies that calling GET /devices/:id
// with a tenant JWT belonging to workspace B is refused with 403 when the device
// belongs to workspace A.
func TestCrossTenant_DeviceA_CannotReadDeviceB(t *testing.T) {
	db := startPostgres(t)

	wsA := uuid.New()
	wsB := uuid.New()

	devAID := insertDevice(t, db, wsA, "sensor-alpha", "ts_keyA001")

	deviceRepo := postgres.NewDeviceRepo(db)
	channelRepo := postgres.NewChannelRepo(db)

	// Sanity check: repo can fetch device A directly.
	fetched, err := deviceRepo.GetByID(context.Background(), devAID)
	require.NoError(t, err)
	assert.Equal(t, wsA, fetched.WorkspaceID)

	// Wire up services — use a no-op cache backed by a disconnected client so that
	// cache errors are silently logged and never surface in this cross-tenant test.
	noopCache := deviceredis.NewDeviceCache(redis.NewClient(&redis.Options{Addr: "localhost:0"}))
	logger := slog.Default()

	deviceSvc := application.NewDeviceService(deviceRepo, noopCache, logger)
	channelSvc := application.NewChannelService(channelRepo)

	deviceHandler := transporthttp.NewDeviceHandler(deviceSvc)
	channelHandler := transporthttp.NewChannelHandler(channelSvc)

	// Request with wsB's token — must get 403.
	router := newTestRouter(wsB.String(), deviceHandler, channelHandler)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, fmt.Sprintf("/devices/%s", devAID), nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusForbidden, w.Code,
		"tenant B must not read tenant A's device; got body: %s", w.Body.String())
}

// TestCrossTenant_ChannelA_CannotBeReadByTenantB verifies that GET /channels/:id
// returns 403 when the channel belongs to workspace A but the caller is workspace B.
func TestCrossTenant_ChannelA_CannotBeReadByTenantB(t *testing.T) {
	db := startPostgres(t)

	wsA := uuid.New()
	wsB := uuid.New()

	chanAID := insertChannel(t, db, wsA, "temperature-feed")

	deviceRepo := postgres.NewDeviceRepo(db)
	channelRepo := postgres.NewChannelRepo(db)

	// Sanity check: repo can fetch channel A.
	fetched, err := channelRepo.GetByID(context.Background(), chanAID)
	require.NoError(t, err)
	assert.Equal(t, wsA, fetched.WorkspaceID)

	noopCache := deviceredis.NewDeviceCache(redis.NewClient(&redis.Options{Addr: "localhost:0"}))
	logger := slog.Default()

	deviceSvc := application.NewDeviceService(deviceRepo, noopCache, logger)
	channelSvc := application.NewChannelService(channelRepo)

	deviceHandler := transporthttp.NewDeviceHandler(deviceSvc)
	channelHandler := transporthttp.NewChannelHandler(channelSvc)

	// Request with wsB's token — must get 403.
	router := newTestRouter(wsB.String(), deviceHandler, channelHandler)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, fmt.Sprintf("/channels/%s", chanAID), nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusForbidden, w.Code,
		"tenant B must not read tenant A's channel; got body: %s", w.Body.String())
}

// TestRotateAPIKey_ClearsDeviceFromCache verifies the full rotate-key flow:
//  1. A device exists in Postgres with a known API key.
//  2. That key is written into Redis cache.
//  3. RotateAPIKey is called through the HTTP handler.
//  4. After rotation the old key is absent from the cache (ErrCacheMiss).
func TestRotateAPIKey_ClearsDeviceFromCache(t *testing.T) {
	db := startPostgres(t)
	redisClient := startRedis(t)
	ctx := context.Background()

	wsA := uuid.New()
	const oldKey = "ts_testkey_rotate"
	devID := insertDevice(t, db, wsA, "rotate-me", oldKey)

	deviceRepo := postgres.NewDeviceRepo(db)
	channelRepo := postgres.NewChannelRepo(db)
	deviceCache := deviceredis.NewDeviceCache(redisClient)
	logger := slog.Default()

	// Pre-populate cache with the device's old API key entry.
	dev, err := deviceRepo.GetByID(ctx, devID)
	require.NoError(t, err)
	require.Equal(t, oldKey, dev.APIKey)

	err = deviceCache.SetDevice(ctx, dev)
	require.NoError(t, err, "seed cache")

	// Confirm old key is present in cache before rotation.
	cached, err := deviceCache.GetDeviceByAPIKey(ctx, oldKey)
	require.NoError(t, err, "old key should be in cache before rotation")
	assert.Equal(t, devID, cached.ID)

	// Wire the full HTTP stack (device handler does ownership check first).
	deviceSvc := application.NewDeviceService(deviceRepo, deviceCache, logger)
	channelSvc := application.NewChannelService(channelRepo)
	deviceHandler := transporthttp.NewDeviceHandler(deviceSvc)
	channelHandler := transporthttp.NewChannelHandler(channelSvc)

	router := newTestRouter(wsA.String(), deviceHandler, channelHandler)

	// Call POST /devices/:id/rotate-key.
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, fmt.Sprintf("/devices/%s/rotate-key", devID), nil)
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code,
		"rotate-key should succeed for device owner; got body: %s", w.Body.String())

	// Old API key must no longer be present in cache.
	_, err = deviceCache.GetDeviceByAPIKey(ctx, oldKey)
	assert.ErrorIs(t, err, device.ErrCacheMiss,
		"old API key should be evicted from cache after rotation")

	// New API key should now be cached.
	updatedDev, err := deviceRepo.GetByID(ctx, devID)
	require.NoError(t, err)
	assert.NotEqual(t, oldKey, updatedDev.APIKey, "API key in DB must have changed")

	cachedNew, err := deviceCache.GetDeviceByAPIKey(ctx, updatedDev.APIKey)
	require.NoError(t, err, "new API key should be present in cache")
	assert.Equal(t, devID, cachedNew.ID)
}
