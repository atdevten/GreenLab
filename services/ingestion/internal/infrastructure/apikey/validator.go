package apikey

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/greenlab/ingestion/internal/domain"
)

// cache abstracts the Redis API key cache.
type cache interface {
	Validate(ctx context.Context, apiKey string) (deviceID, channelID string, err error)
	Set(ctx context.Context, apiKey, deviceID, channelID string) error
}

// store abstracts the Postgres device store.
type store interface {
	GetByAPIKey(ctx context.Context, apiKey string) (deviceID, channelID string, err error)
}

// Validator validates API keys via Redis, falling back to Postgres on cache miss.
type Validator struct {
	cache  cache
	store  store
	logger *slog.Logger
}

func NewValidator(cache cache, store store, logger *slog.Logger) *Validator {
	return &Validator{cache: cache, store: store, logger: logger}
}

func (v *Validator) Validate(ctx context.Context, apiKey string) (deviceID, channelID string, err error) {
	deviceID, channelID, err = v.cache.Validate(ctx, apiKey)
	if err == nil {
		return deviceID, channelID, nil
	}
	if !errors.Is(err, domain.ErrCacheMiss) {
		v.logger.Error("api key cache lookup failed", "error", err)
	}

	// cache miss — fall back to Postgres
	deviceID, channelID, err = v.store.GetByAPIKey(ctx, apiKey)
	if err != nil {
		return "", "", fmt.Errorf("Validator.store: %w", err)
	}

	// populate cache for subsequent requests (non-fatal)
	if err := v.cache.Set(ctx, apiKey, deviceID, channelID); err != nil {
		v.logger.Error("failed to cache api key", "error", err)
	}
	return deviceID, channelID, nil
}
