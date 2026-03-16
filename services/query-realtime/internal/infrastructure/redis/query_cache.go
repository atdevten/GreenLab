package redis

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"
)

// QueryCache implements the application.Cache interface using Redis.
type QueryCache struct {
	rdb *redis.Client
}

// NewQueryCache creates a new QueryCache backed by the given Redis client.
func NewQueryCache(rdb *redis.Client) *QueryCache {
	return &QueryCache{rdb: rdb}
}

// Get retrieves a cached value by key. Returns redis.Nil if not found.
func (c *QueryCache) Get(ctx context.Context, key string) ([]byte, error) {
	return c.rdb.Get(ctx, key).Bytes()
}

// Set stores a value under key with the given TTL.
func (c *QueryCache) Set(ctx context.Context, key string, value []byte, ttl time.Duration) error {
	return c.rdb.Set(ctx, key, value, ttl).Err()
}
