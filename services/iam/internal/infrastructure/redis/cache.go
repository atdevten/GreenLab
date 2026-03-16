package redis

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

type Cache struct {
	client *redis.Client
}

func NewCache(client *redis.Client) *Cache { return &Cache{client: client} }

func (c *Cache) SetUserSession(ctx context.Context, userID string, data interface{}, ttlSeconds int) error {
	b, err := json.Marshal(data)
	if err != nil {
		return err
	}
	return c.client.Set(ctx, fmt.Sprintf("session:%s", userID), b, time.Duration(ttlSeconds)*time.Second).Err()
}

func (c *Cache) GetUserSession(ctx context.Context, userID string) ([]byte, error) {
	val, err := c.client.Get(ctx, fmt.Sprintf("session:%s", userID)).Bytes()
	if errors.Is(err, redis.Nil) {
		return nil, fmt.Errorf("session not found")
	}
	return val, err
}

func (c *Cache) DeleteUserSession(ctx context.Context, userID string) error {
	return c.client.Del(ctx, fmt.Sprintf("session:%s", userID)).Err()
}

func (c *Cache) BlacklistToken(ctx context.Context, jti string, ttlSeconds int) error {
	return c.client.Set(ctx, fmt.Sprintf("blacklist:jti:%s", jti), "1", time.Duration(ttlSeconds)*time.Second).Err()
}

func (c *Cache) IsTokenBlacklisted(ctx context.Context, jti string) (bool, error) {
	val, err := c.client.Get(ctx, fmt.Sprintf("blacklist:jti:%s", jti)).Result()
	if errors.Is(err, redis.Nil) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return val == "1", nil
}
