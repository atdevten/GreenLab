package redis

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

type Config struct {
	Addr     string
	Password string
	DB       int
}

// Connect opens and pings a Redis connection, fatal on failure.
func Connect(log *zap.Logger, cfg Config) *redis.Client {
	rdb := redis.NewClient(&redis.Options{
		Addr:     cfg.Addr,
		Password: cfg.Password,
		DB:       cfg.DB,
	})
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := rdb.Ping(ctx).Err(); err != nil {
		log.Fatal("redis connect failed", zap.Error(err))
	}
	log.Info("connected to redis")
	return rdb
}
