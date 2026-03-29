package redis

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/greenlab/ingestion/internal/domain"
)

const apiKeyCacheTTL = 10 * time.Minute

// APIKeyCache caches API key → device/channel lookups in Redis.
type APIKeyCache struct {
	client *redis.Client
}

func NewAPIKeyCache(client *redis.Client) *APIKeyCache {
	return &APIKeyCache{client: client}
}

// cacheKey hashes the API key + channelID pair so credentials are never stored in Redis.
func cacheKey(apiKey, channelID string) string {
	sum := sha256.Sum256([]byte(apiKey + ":" + channelID))
	return "apikey:" + hex.EncodeToString(sum[:])
}

// deviceVersionKey returns the Redis key used by device-registry to signal
// that a device's API key or state has changed.
func deviceVersionKey(deviceID string) string {
	return fmt.Sprintf("device_version:%s", deviceID)
}

// cachedEntry is the JSON representation stored in Redis.
// Version mirrors the value of device_version:{deviceID} at write time.
type cachedEntry struct {
	Schema  domain.DeviceSchema `json:"schema"`
	Version int64               `json:"version"`
}

// Validate looks up a device schema by API key + channelID from the cache.
// On a cache hit it also reads device_version:{deviceID} and compares it
// against the stored version — returning ErrCacheMiss if they differ so the
// caller re-validates against device-registry.
// Returns domain.ErrCacheMiss if the entry is not cached, cannot be decoded,
// or the version is stale.
func (c *APIKeyCache) Validate(ctx context.Context, apiKey, channelID string) (domain.DeviceSchema, error) {
	val, err := c.client.Get(ctx, cacheKey(apiKey, channelID)).Bytes()
	if errors.Is(err, redis.Nil) {
		return domain.DeviceSchema{}, domain.ErrCacheMiss
	}
	if err != nil {
		return domain.DeviceSchema{}, fmt.Errorf("APIKeyCache.Validate: %w", err)
	}

	var entry cachedEntry
	if err := json.Unmarshal(val, &entry); err != nil {
		// Graceful degradation: old cache entries (pre-refactor) won't decode into cachedEntry.
		// Treat as a cache miss so the caller re-fetches from device-registry.
		return domain.DeviceSchema{}, domain.ErrCacheMiss
	}

	// Version check: fetch the current device version counter and compare.
	currentVersionStr, err := c.client.Get(ctx, deviceVersionKey(entry.Schema.DeviceID)).Result()
	if err != nil && !errors.Is(err, redis.Nil) {
		// If we cannot read the version key (transient Redis error), treat as miss.
		return domain.DeviceSchema{}, domain.ErrCacheMiss
	}
	if err == nil {
		// Version key exists — compare.
		currentVersion, parseErr := strconv.ParseInt(currentVersionStr, 10, 64)
		if parseErr == nil && currentVersion != entry.Version {
			// Stale entry: device API key was rotated or device was deleted.
			return domain.DeviceSchema{}, domain.ErrCacheMiss
		}
	}
	// If the version key does not exist (redis.Nil), the device has never been
	// invalidated — treat the cached entry as fresh.

	return entry.Schema, nil
}

// Set stores an API key + channelID → DeviceSchema mapping together with
// the current device version counter so stale checks work on future hits.
func (c *APIKeyCache) Set(ctx context.Context, apiKey, channelID string, schema domain.DeviceSchema) error {
	// Read the current version (0 if not yet set).
	var version int64
	vStr, err := c.client.Get(ctx, deviceVersionKey(schema.DeviceID)).Result()
	if err == nil {
		version, _ = strconv.ParseInt(vStr, 10, 64)
	}

	entry := cachedEntry{Schema: schema, Version: version}
	b, err := json.Marshal(entry)
	if err != nil {
		return fmt.Errorf("APIKeyCache.Set marshal: %w", err)
	}
	if err := c.client.Set(ctx, cacheKey(apiKey, channelID), b, apiKeyCacheTTL).Err(); err != nil {
		return fmt.Errorf("APIKeyCache.Set: %w", err)
	}
	return nil
}

// Delete removes a cached entry for an API key + channelID pair.
func (c *APIKeyCache) Delete(ctx context.Context, apiKey, channelID string) error {
	if err := c.client.Del(ctx, cacheKey(apiKey, channelID)).Err(); err != nil {
		return fmt.Errorf("APIKeyCache.Delete: %w", err)
	}
	return nil
}
