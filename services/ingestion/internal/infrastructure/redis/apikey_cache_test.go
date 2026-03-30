package redis

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/go-redis/redismock/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/greenlab/ingestion/internal/domain"
)

func makeSchema(deviceID string) domain.DeviceSchema {
	return domain.DeviceSchema{
		DeviceID:      deviceID,
		ChannelID:     "chan-1",
		Fields:        []domain.FieldEntry{{Index: 1, Name: "temperature", Type: "float"}},
		SchemaVersion: 1,
	}
}

// encodedEntry builds the JSON bytes that the cache would store for a schema+version pair.
func encodedEntry(schema domain.DeviceSchema, version int64) []byte {
	b, _ := json.Marshal(cachedEntry{Schema: schema, Version: version})
	return b
}

func TestAPIKeyCache_Validate_CacheMiss(t *testing.T) {
	client, mock := redismock.NewClientMock()
	cache := NewAPIKeyCache(client)
	ctx := context.Background()

	mock.ExpectGet(cacheKey("key", "chan")).RedisNil()

	_, err := cache.Validate(ctx, "key", "chan")
	assert.ErrorIs(t, err, domain.ErrCacheMiss)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestAPIKeyCache_Validate_HappyPath_VersionMatch(t *testing.T) {
	client, mock := redismock.NewClientMock()
	cache := NewAPIKeyCache(client)
	ctx := context.Background()

	deviceID := "dev-abc"
	schema := makeSchema(deviceID)
	version := int64(3)

	mock.ExpectGet(cacheKey("key", "chan")).SetVal(string(encodedEntry(schema, version)))
	mock.ExpectGet(deviceVersionKey(deviceID)).SetVal("3")

	got, err := cache.Validate(ctx, "key", "chan")
	require.NoError(t, err)
	assert.Equal(t, schema, got)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestAPIKeyCache_Validate_StaleVersion_ReturnsCacheMiss(t *testing.T) {
	client, mock := redismock.NewClientMock()
	cache := NewAPIKeyCache(client)
	ctx := context.Background()

	deviceID := "dev-abc"
	schema := makeSchema(deviceID)
	// Stored version is 3, but device-registry has already incremented to 4.
	mock.ExpectGet(cacheKey("key", "chan")).SetVal(string(encodedEntry(schema, 3)))
	mock.ExpectGet(deviceVersionKey(deviceID)).SetVal("4")

	_, err := cache.Validate(ctx, "key", "chan")
	assert.ErrorIs(t, err, domain.ErrCacheMiss)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestAPIKeyCache_Validate_NoVersionKey_TreatedAsFresh(t *testing.T) {
	// If device_version:{id} doesn't exist yet (never rotated/deleted),
	// the cached entry should still be returned as valid.
	client, mock := redismock.NewClientMock()
	cache := NewAPIKeyCache(client)
	ctx := context.Background()

	deviceID := "dev-new"
	schema := makeSchema(deviceID)

	mock.ExpectGet(cacheKey("key", "chan")).SetVal(string(encodedEntry(schema, 0)))
	mock.ExpectGet(deviceVersionKey(deviceID)).RedisNil()

	got, err := cache.Validate(ctx, "key", "chan")
	require.NoError(t, err)
	assert.Equal(t, schema, got)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestAPIKeyCache_Validate_MalformedJSON_ReturnsCacheMiss(t *testing.T) {
	client, mock := redismock.NewClientMock()
	cache := NewAPIKeyCache(client)
	ctx := context.Background()

	mock.ExpectGet(cacheKey("key", "chan")).SetVal("not-json")

	_, err := cache.Validate(ctx, "key", "chan")
	assert.ErrorIs(t, err, domain.ErrCacheMiss)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestAPIKeyCache_Set_StoresVersionSnapshot(t *testing.T) {
	client, mock := redismock.NewClientMock()
	cache := NewAPIKeyCache(client)
	ctx := context.Background()

	deviceID := "dev-abc"
	schema := makeSchema(deviceID)

	// Simulate device_version already at 2 when we cache.
	mock.ExpectGet(deviceVersionKey(deviceID)).SetVal("2")
	expectedEntry := encodedEntry(schema, 2)
	mock.ExpectSet(cacheKey("key", "chan"), expectedEntry, apiKeyCacheTTL).SetVal("OK")

	err := cache.Set(ctx, "key", "chan", schema)
	require.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestAPIKeyCache_Set_VersionKeyAbsent_StoresZero(t *testing.T) {
	client, mock := redismock.NewClientMock()
	cache := NewAPIKeyCache(client)
	ctx := context.Background()

	deviceID := "dev-new"
	schema := makeSchema(deviceID)

	mock.ExpectGet(deviceVersionKey(deviceID)).RedisNil()
	expectedEntry := encodedEntry(schema, 0) // version defaults to 0
	mock.ExpectSet(cacheKey("key", "chan"), expectedEntry, apiKeyCacheTTL).SetVal("OK")

	err := cache.Set(ctx, "key", "chan", schema)
	require.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestAPIKeyCache_Delete(t *testing.T) {
	client, mock := redismock.NewClientMock()
	cache := NewAPIKeyCache(client)
	ctx := context.Background()

	mock.ExpectDel(cacheKey("key", "chan")).SetVal(1)

	err := cache.Delete(ctx, "key", "chan")
	require.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestAPIKeyCache_Validate_RedisTransientError_ReturnsCacheMiss(t *testing.T) {
	// If reading the version key fails with a non-Nil Redis error, treat as miss.
	client, mock := redismock.NewClientMock()
	cache := NewAPIKeyCache(client)
	ctx := context.Background()

	deviceID := "dev-abc"
	schema := makeSchema(deviceID)

	mock.ExpectGet(cacheKey("key", "chan")).SetVal(string(encodedEntry(schema, 3)))
	mock.ExpectGet(deviceVersionKey(deviceID)).SetErr(assert.AnError)

	_, err := cache.Validate(ctx, "key", "chan")
	assert.ErrorIs(t, err, domain.ErrCacheMiss)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestAPIKeyCache_Validate_CorruptVersionKey_ReturnsCacheMiss(t *testing.T) {
	// If the version key contains non-numeric data, treat as cache miss
	// (fail closed rather than silently serving a potentially stale entry).
	client, mock := redismock.NewClientMock()
	cache := NewAPIKeyCache(client)
	ctx := context.Background()

	deviceID := "dev-abc"
	schema := makeSchema(deviceID)

	mock.ExpectGet(cacheKey("key", "chan")).SetVal(string(encodedEntry(schema, 3)))
	mock.ExpectGet(deviceVersionKey(deviceID)).SetVal("not-a-number")

	_, err := cache.Validate(ctx, "key", "chan")
	assert.ErrorIs(t, err, domain.ErrCacheMiss)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// Verify the TTL constant is sensible (not accidentally zero).
func TestAPIKeyCacheTTL(t *testing.T) {
	assert.Equal(t, 10*time.Minute, apiKeyCacheTTL)
}
