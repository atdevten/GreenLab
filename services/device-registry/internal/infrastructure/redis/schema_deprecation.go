package redis

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

const forceDeprecatedTTL = 48 * time.Hour

// SchemaDeprecationStore manages force-deprecation markers for channel schema versions.
//
// Key schema:
//   schema_force_deprecated:{channel_id} — Unix timestamp of deprecation, TTL 48 hours
//
// Cleanup is handled automatically by Redis TTL — no explicit cleanup job is needed.
type SchemaDeprecationStore struct {
	client *redis.Client
}

// NewSchemaDeprecationStore creates a SchemaDeprecationStore backed by the given Redis client.
func NewSchemaDeprecationStore(client *redis.Client) *SchemaDeprecationStore {
	return &SchemaDeprecationStore{client: client}
}

func forceDeprecatedKey(channelID string) string {
	return fmt.Sprintf("schema_force_deprecated:%s", channelID)
}

// SetForceDeprecated sets the force-deprecation marker for the given channelID.
// The key stores the current Unix timestamp and expires after 48 hours.
// After expiry, the deprecation is considered lifted and devices are no longer blocked.
func (s *SchemaDeprecationStore) SetForceDeprecated(ctx context.Context, channelID string) error {
	key := forceDeprecatedKey(channelID)
	if err := s.client.Set(ctx, key, time.Now().Unix(), forceDeprecatedTTL).Err(); err != nil {
		return fmt.Errorf("SchemaDeprecationStore.SetForceDeprecated: %w", err)
	}
	return nil
}

// IsForceDeprecated returns true if a force-deprecation marker exists for channelID.
// Returns false if the key has expired or was never set.
func (s *SchemaDeprecationStore) IsForceDeprecated(ctx context.Context, channelID string) (bool, error) {
	err := s.client.Get(ctx, forceDeprecatedKey(channelID)).Err()
	if err == redis.Nil {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("SchemaDeprecationStore.IsForceDeprecated: %w", err)
	}
	return true, nil
}
