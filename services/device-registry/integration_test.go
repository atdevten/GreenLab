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
  short_id     SERIAL UNIQUE,
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

CREATE TABLE IF NOT EXISTS fields (
  id          UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
  channel_id  UUID NOT NULL REFERENCES channels(id) ON DELETE CASCADE,
  name        TEXT NOT NULL,
  label       TEXT NOT NULL DEFAULT '',
  unit        TEXT NOT NULL DEFAULT '',
  field_type  TEXT NOT NULL DEFAULT 'float',
  position    INTEGER NOT NULL DEFAULT 1,
  description TEXT NOT NULL DEFAULT '',
  created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  UNIQUE (channel_id, position)
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

// noopDeviceCache is a DeviceCacheRepository that silently succeeds all operations.
// Used in tests that exercise the HTTP/domain layer without needing a real Redis,
// avoiding connection errors from a disconnected client.
type noopDeviceCache struct{}

func (n *noopDeviceCache) SetDevice(_ context.Context, _ *device.Device) error {
	return nil
}
func (n *noopDeviceCache) GetDeviceByAPIKey(_ context.Context, _ string) (*device.Device, error) {
	return nil, device.ErrCacheMiss
}
func (n *noopDeviceCache) DeleteDevice(_ context.Context, _, _ string) error {
	return nil
}
func (n *noopDeviceCache) IncrDeviceVersion(_ context.Context, _ string) error {
	return nil
}

// noopRetentionManager is a RetentionManager that silently succeeds.
// Used in tests that don't need real InfluxDB retention policy management.
type noopRetentionManager struct{}

func (n *noopRetentionManager) SetRetention(_ context.Context, _ string, _ int) error {
	return nil
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

// insertChannelForDevice inserts a channel associated with a device and returns its UUID.
func insertChannelForDevice(t *testing.T, db *sqlx.DB, workspaceID, deviceID uuid.UUID, name string) uuid.UUID {
	t.Helper()
	id := uuid.New()
	_, err := db.ExecContext(context.Background(), `
		INSERT INTO channels (id, workspace_id, device_id, name, visibility, tags)
		VALUES ($1, $2, $3, $4, 'private', '[]')`,
		id, workspaceID, deviceID, name,
	)
	require.NoError(t, err, "insert channel %s for device", name)
	return id
}

// insertField inserts a field row for the given channel and returns its UUID.
func insertField(t *testing.T, db *sqlx.DB, channelID uuid.UUID, name, fieldType string, position int) uuid.UUID {
	t.Helper()
	id := uuid.New()
	_, err := db.ExecContext(context.Background(), `
		INSERT INTO fields (id, channel_id, name, field_type, position)
		VALUES ($1, $2, $3, $4, $5)`,
		id, channelID, name, fieldType, position,
	)
	require.NoError(t, err, "insert field %s", name)
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

// TestGetDevice_ReturnsDeviceByID verifies that GET /devices/:id returns the device
// for any authenticated caller. Tenant-scoping on individual resource reads is enforced
// upstream (API gateway / auth middleware); the device-registry handler returns by ID.
func TestGetDevice_ReturnsDeviceByID(t *testing.T) {
	db := startPostgres(t)

	wsA := uuid.New()
	devAID := insertDevice(t, db, wsA, "sensor-alpha", "ts_keyA001")

	deviceRepo := postgres.NewDeviceRepo(db)
	channelRepo := postgres.NewChannelRepo(db)

	logger := slog.Default()
	deviceSvc := application.NewDeviceService(deviceRepo, &noopDeviceCache{}, logger)
	channelSvc := application.NewChannelService(channelRepo, &noopRetentionManager{}, slog.Default())

	deviceHandler := transporthttp.NewDeviceHandler(deviceSvc)
	channelHandler := transporthttp.NewChannelHandler(channelSvc)

	router := newTestRouter(wsA.String(), deviceHandler, channelHandler)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, fmt.Sprintf("/devices/%s", devAID), nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code,
		"device owner should be able to read their device; got body: %s", w.Body.String())
}

// TestGetChannel_ReturnsChannelByID verifies that GET /channels/:id returns the channel.
// Tenant-scoping on individual resource reads is enforced upstream (API gateway / auth
// middleware); the device-registry handler returns by ID.
func TestGetChannel_ReturnsChannelByID(t *testing.T) {
	db := startPostgres(t)

	wsA := uuid.New()
	chanAID := insertChannel(t, db, wsA, "temperature-feed")

	deviceRepo := postgres.NewDeviceRepo(db)
	channelRepo := postgres.NewChannelRepo(db)

	logger := slog.Default()
	deviceSvc := application.NewDeviceService(deviceRepo, &noopDeviceCache{}, logger)
	channelSvc := application.NewChannelService(channelRepo, &noopRetentionManager{}, slog.Default())

	deviceHandler := transporthttp.NewDeviceHandler(deviceSvc)
	channelHandler := transporthttp.NewChannelHandler(channelSvc)

	router := newTestRouter(wsA.String(), deviceHandler, channelHandler)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, fmt.Sprintf("/channels/%s", chanAID), nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code,
		"channel owner should be able to read their channel; got body: %s", w.Body.String())
}

// TestRotateAPIKey_UpdatesKeyAndIncreasesVersion verifies the full rotate-key flow:
//  1. A device exists in Postgres with a known API key.
//  2. That key is written into Redis cache.
//  3. RotateAPIKey is called through the HTTP handler.
//  4. After rotation the DB has a new key and the new key is in cache.
//  5. The device version counter is incremented (signals staleness to downstream consumers).
func TestRotateAPIKey_UpdatesKeyAndIncreasesVersion(t *testing.T) {
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

	// Wire the full HTTP stack.
	deviceSvc := application.NewDeviceService(deviceRepo, deviceCache, logger)
	channelSvc := application.NewChannelService(channelRepo, &noopRetentionManager{}, slog.Default())
	deviceHandler := transporthttp.NewDeviceHandler(deviceSvc)
	channelHandler := transporthttp.NewChannelHandler(channelSvc)

	router := newTestRouter(wsA.String(), deviceHandler, channelHandler)

	// Call POST /devices/:id/rotate-key.
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, fmt.Sprintf("/devices/%s/rotate-key", devID), nil)
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code,
		"rotate-key should succeed; got body: %s", w.Body.String())

	// DB must have a new API key.
	updatedDev, err := deviceRepo.GetByID(ctx, devID)
	require.NoError(t, err)
	assert.NotEqual(t, oldKey, updatedDev.APIKey, "API key in DB must have changed")

	// New key must be cached (SetDevice is called with the rotated device).
	cachedNew, err := deviceCache.GetDeviceByAPIKey(ctx, updatedDev.APIKey)
	require.NoError(t, err, "new API key should be present in cache")
	assert.Equal(t, devID, cachedNew.ID)

	// Device version must be incremented — signals staleness to downstream consumers
	// (e.g. ingestion cache) without needing to delete the old key directly.
	versionKey := fmt.Sprintf("device_version:%s", devID.String())
	versionStr, err := redisClient.Get(ctx, versionKey).Result()
	require.NoError(t, err, "device_version key should exist after rotation")
	assert.Equal(t, "1", versionStr, "version should be incremented to 1 after first rotation")
}

// ─── InternalRepo.ResolveChannelByAPIKey ────────────────────────────────────

func TestInternalRepo_ResolveChannelByAPIKey(t *testing.T) {
	db := startPostgres(t)
	repo := postgres.NewInternalRepo(db)
	ctx := context.Background()
	wsID := uuid.New()

	t.Run("active device with fields returns correct schema", func(t *testing.T) {
		devID := insertDevice(t, db, wsID, "dev-resolve-1", "key-resolve-1")
		chanID := insertChannelForDevice(t, db, wsID, devID, "chan-resolve-1")
		insertField(t, db, chanID, "temperature", "float", 1)
		insertField(t, db, chanID, "humidity", "float", 2)

		result, err := repo.ResolveChannelByAPIKey(ctx, "key-resolve-1")
		require.NoError(t, err)
		assert.Equal(t, devID.String(), result.DeviceID)
		assert.Equal(t, chanID.String(), result.ChannelID)
		require.Len(t, result.Fields, 2)
		assert.Equal(t, "temperature", result.Fields[0].Name)
		assert.Equal(t, "humidity", result.Fields[1].Name)
	})

	t.Run("multi-channel device returns only first channel's fields", func(t *testing.T) {
		devID := insertDevice(t, db, wsID, "dev-multichan", "key-multichan")
		chan1ID := insertChannelForDevice(t, db, wsID, devID, "chan-first")
		chan2ID := insertChannelForDevice(t, db, wsID, devID, "chan-second")
		insertField(t, db, chan1ID, "voltage", "float", 1)
		insertField(t, db, chan2ID, "pressure", "float", 1) // same position, different channel

		result, err := repo.ResolveChannelByAPIKey(ctx, "key-multichan")
		require.NoError(t, err)
		assert.Equal(t, chan1ID.String(), result.ChannelID, "should resolve to oldest channel")
		require.Len(t, result.Fields, 1, "must not include fields from other channels")
		assert.Equal(t, "voltage", result.Fields[0].Name)
	})

	t.Run("device with no fields returns empty fields slice", func(t *testing.T) {
		devID := insertDevice(t, db, wsID, "dev-nofields", "key-nofields")
		chanID := insertChannelForDevice(t, db, wsID, devID, "chan-nofields")
		_ = chanID

		result, err := repo.ResolveChannelByAPIKey(ctx, "key-nofields")
		require.NoError(t, err)
		assert.NotNil(t, result.Fields)
		assert.Empty(t, result.Fields)
	})

	t.Run("inactive device returns ErrDeviceNotFound", func(t *testing.T) {
		inactiveID := uuid.New()
		_, err := db.ExecContext(ctx, `
			INSERT INTO devices (id, workspace_id, name, api_key, status)
			VALUES ($1, $2, 'dev-inactive', 'key-inactive', 'inactive')`,
			inactiveID, wsID,
		)
		require.NoError(t, err)

		_, err = repo.ResolveChannelByAPIKey(ctx, "key-inactive")
		require.Error(t, err)
		assert.ErrorIs(t, err, device.ErrDeviceNotFound)
	})

	t.Run("unknown api_key returns ErrDeviceNotFound", func(t *testing.T) {
		_, err := repo.ResolveChannelByAPIKey(ctx, "key-does-not-exist")
		require.Error(t, err)
		assert.ErrorIs(t, err, device.ErrDeviceNotFound)
	})
}

// TestResolveChannel_HTTPEndpoint exercises GET /internal/resolve-channel end-to-end.
func TestResolveChannel_HTTPEndpoint(t *testing.T) {
	db := startPostgres(t)
	wsID := uuid.New()

	internalSvc := application.NewInternalService(postgres.NewInternalRepo(db))
	internalHandler := transporthttp.NewInternalHandler(internalSvc)

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/internal/resolve-channel", internalHandler.ResolveChannel)

	t.Run("valid api_key returns 200 with device and channel", func(t *testing.T) {
		devID := insertDevice(t, db, wsID, "dev-http-resolve", "key-http-resolve")
		chanID := insertChannelForDevice(t, db, wsID, devID, "chan-http-resolve")
		insertField(t, db, chanID, "co2", "float", 1)

		req := httptest.NewRequest(http.MethodGet, "/internal/resolve-channel?api_key=key-http-resolve", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		require.Equal(t, http.StatusOK, w.Code)
		assert.Contains(t, w.Body.String(), devID.String())
		assert.Contains(t, w.Body.String(), chanID.String())
		assert.Contains(t, w.Body.String(), "co2")
	})

	t.Run("missing api_key returns 400", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/internal/resolve-channel", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("unknown api_key returns 401", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/internal/resolve-channel?api_key=bad-key", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})
}
