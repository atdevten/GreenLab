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
	Validate(ctx context.Context, apiKey, channelID string) (domain.DeviceSchema, error)
	Set(ctx context.Context, apiKey, channelID string, schema domain.DeviceSchema) error
}

// store abstracts the device-registry HTTP client (or any store-side lookup).
type store interface {
	GetByAPIKey(ctx context.Context, apiKey, channelID string) (domain.DeviceSchema, error)
}

// Validator validates API keys via Redis, falling back to device-registry on cache miss.
type Validator struct {
	cache  cache
	store  store
	logger *slog.Logger
}

func NewValidator(cache cache, store store, logger *slog.Logger) *Validator {
	return &Validator{cache: cache, store: store, logger: logger}
}

func (v *Validator) Validate(ctx context.Context, apiKey, channelID string) (domain.DeviceSchema, error) {
	schema, err := v.cache.Validate(ctx, apiKey, channelID)
	if err == nil {
		return schema, nil
	}
	if !errors.Is(err, domain.ErrCacheMiss) {
		v.logger.Error("api key cache lookup failed", "error", err)
	}

	// cache miss — fall back to device-registry
	schema, err = v.store.GetByAPIKey(ctx, apiKey, channelID)
	if err != nil {
		return domain.DeviceSchema{}, fmt.Errorf("Validator.store: %w", err)
	}

	// populate cache for subsequent requests (non-fatal)
	if err := v.cache.Set(ctx, apiKey, channelID, schema); err != nil {
		v.logger.Error("failed to cache api key", "error", err)
	}
	return schema, nil
}
