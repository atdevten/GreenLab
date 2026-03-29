package redis

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
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

// Validate looks up a device schema by API key + channelID from the cache.
// Returns domain.ErrCacheMiss if the entry is not cached or cannot be decoded.
func (c *APIKeyCache) Validate(ctx context.Context, apiKey, channelID string) (domain.DeviceSchema, error) {
	val, err := c.client.Get(ctx, cacheKey(apiKey, channelID)).Bytes()
	if errors.Is(err, redis.Nil) {
		return domain.DeviceSchema{}, domain.ErrCacheMiss
	}
	if err != nil {
		return domain.DeviceSchema{}, fmt.Errorf("APIKeyCache.Validate: %w", err)
	}
	var schema domain.DeviceSchema
	if err := json.Unmarshal(val, &schema); err != nil {
		// Graceful degradation: old cache entries (pre-refactor) won't decode into DeviceSchema.
		// Treat as a cache miss so the caller re-fetches from device-registry.
		return domain.DeviceSchema{}, domain.ErrCacheMiss
	}
	return schema, nil
}

// Set stores an API key + channelID → DeviceSchema mapping.
func (c *APIKeyCache) Set(ctx context.Context, apiKey, channelID string, schema domain.DeviceSchema) error {
	b, err := json.Marshal(schema)
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
