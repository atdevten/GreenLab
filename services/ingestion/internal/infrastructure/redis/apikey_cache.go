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

type deviceInfo struct {
	DeviceID  string `json:"device_id"`
	ChannelID string `json:"channel_id"`
}

// APIKeyCache caches API key → device/channel lookups in Redis.
type APIKeyCache struct {
	client *redis.Client
}

func NewAPIKeyCache(client *redis.Client) *APIKeyCache {
	return &APIKeyCache{client: client}
}

// cacheKey hashes the raw API key so credentials are never stored in the Redis keyspace.
func cacheKey(apiKey string) string {
	sum := sha256.Sum256([]byte(apiKey))
	return "apikey:" + hex.EncodeToString(sum[:])
}

// Validate looks up a device by API key from the cache.
// Returns domain.ErrCacheMiss if the key is not cached.
func (c *APIKeyCache) Validate(ctx context.Context, apiKey string) (deviceID, channelID string, err error) {
	val, err := c.client.Get(ctx, cacheKey(apiKey)).Bytes()
	if errors.Is(err, redis.Nil) {
		return "", "", domain.ErrCacheMiss
	}
	if err != nil {
		return "", "", fmt.Errorf("APIKeyCache.Validate: %w", err)
	}
	var info deviceInfo
	if err := json.Unmarshal(val, &info); err != nil {
		return "", "", fmt.Errorf("APIKeyCache.Validate unmarshal: %w", err)
	}
	return info.DeviceID, info.ChannelID, nil
}

// Set stores an API key → device/channel mapping.
func (c *APIKeyCache) Set(ctx context.Context, apiKey, deviceID, channelID string) error {
	b, err := json.Marshal(deviceInfo{DeviceID: deviceID, ChannelID: channelID})
	if err != nil {
		return fmt.Errorf("APIKeyCache.Set marshal: %w", err)
	}
	if err := c.client.Set(ctx, cacheKey(apiKey), b, apiKeyCacheTTL).Err(); err != nil {
		return fmt.Errorf("APIKeyCache.Set: %w", err)
	}
	return nil
}

// Delete removes an API key from the cache.
func (c *APIKeyCache) Delete(ctx context.Context, apiKey string) error {
	if err := c.client.Del(ctx, cacheKey(apiKey)).Err(); err != nil {
		return fmt.Errorf("APIKeyCache.Delete: %w", err)
	}
	return nil
}
