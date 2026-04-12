package redis

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
)

const (
	schemaACKTTL    = 30 * 24 * time.Hour // 30 days
	schemaActiveTTL = 7 * 24 * time.Hour  // 7 days
	schemaStuckTTL  = 48 * time.Hour      // mirrors force-deprecation window
)

// SchemaACKStore records per-device schema version acknowledgements in Redis.
//
// Key schema:
//   schema_ack:{channel_id}:{device_id}    — highest ACK'd schema version (uint32 as string), TTL 30 days
//   schema_active:{channel_id}:{device_id} — Unix timestamp of last active request, TTL 7 days
//
// Cleanup is handled automatically by Redis TTL — no explicit cleanup job is needed.
type SchemaACKStore struct {
	client *redis.Client
}

// NewSchemaACKStore creates a new SchemaACKStore backed by the given Redis client.
func NewSchemaACKStore(client *redis.Client) *SchemaACKStore {
	return &SchemaACKStore{client: client}
}

func schemaACKKey(channelID, deviceID string) string {
	return fmt.Sprintf("schema_ack:%s:%s", channelID, deviceID)
}

func schemaActiveKey(channelID, deviceID string) string {
	return fmt.Sprintf("schema_active:%s:%s", channelID, deviceID)
}

func schemaForceDeprecatedKey(channelID string) string {
	return fmt.Sprintf("schema_force_deprecated:%s", channelID)
}

func schemaStuckKey(channelID, deviceID string) string {
	return fmt.Sprintf("schema_stuck:%s:%s", channelID, deviceID)
}

// RecordACK records that deviceID on channelID has successfully ingested the given schema version.
// It updates schema_ack:{channel_id}:{device_id} to max(current, version) with a 30-day TTL,
// and sets schema_active:{channel_id}:{device_id} to the current Unix timestamp with a 7-day TTL.
// Prevents schema version downgrade: only updates if the new version is higher than the stored one.
func (s *SchemaACKStore) RecordACK(ctx context.Context, channelID, deviceID string, version uint32) error {
	ackKey := schemaACKKey(channelID, deviceID)
	activeKey := schemaActiveKey(channelID, deviceID)

	// GET current ACK'd version and only SET if new version is higher (no downgrade).
	currentStr, err := s.client.Get(ctx, ackKey).Result()
	if err != nil && !errors.Is(err, redis.Nil) {
		return fmt.Errorf("SchemaACKStore.RecordACK get: %w", err)
	}

	if err == nil {
		current, parseErr := strconv.ParseUint(currentStr, 10, 32)
		if parseErr == nil && uint32(current) >= version {
			// Current stored version is already >= new version — update active key only.
			if setErr := s.client.Set(ctx, activeKey, time.Now().Unix(), schemaActiveTTL).Err(); setErr != nil {
				return fmt.Errorf("SchemaACKStore.RecordACK set active: %w", setErr)
			}
			return nil
		}
	}

	// Write the new version and mark the device as active in a pipeline.
	pipe := s.client.Pipeline()
	pipe.Set(ctx, ackKey, strconv.FormatUint(uint64(version), 10), schemaACKTTL)
	pipe.Set(ctx, activeKey, time.Now().Unix(), schemaActiveTTL)
	if _, err := pipe.Exec(ctx); err != nil {
		return fmt.Errorf("SchemaACKStore.RecordACK pipeline: %w", err)
	}
	return nil
}

// ACKedVersion returns the highest schema version ACK'd by deviceID on channelID.
// Returns 0 if no ACK has been recorded.
func (s *SchemaACKStore) ACKedVersion(ctx context.Context, channelID, deviceID string) (uint32, error) {
	val, err := s.client.Get(ctx, schemaACKKey(channelID, deviceID)).Result()
	if errors.Is(err, redis.Nil) {
		return 0, nil
	}
	if err != nil {
		return 0, fmt.Errorf("SchemaACKStore.ACKedVersion: %w", err)
	}
	v, err := strconv.ParseUint(val, 10, 32)
	if err != nil {
		return 0, fmt.Errorf("SchemaACKStore.ACKedVersion parse: %w", err)
	}
	return uint32(v), nil
}

// ActiveDeviceCount returns the number of devices that have been active (made a request)
// in the last 7 days on channelID, by scanning schema_active:{channelID}:* keys.
func (s *SchemaACKStore) ActiveDeviceCount(ctx context.Context, channelID string) (int64, error) {
	pattern := fmt.Sprintf("schema_active:%s:*", channelID)
	count, err := s.scanCount(ctx, pattern)
	if err != nil {
		return 0, fmt.Errorf("SchemaACKStore.ActiveDeviceCount: %w", err)
	}
	return count, nil
}

// ACKedDeviceCount returns the number of active devices that have ACK'd at least the given
// schema version on channelID. It scans schema_ack:{channelID}:* and counts keys whose
// stored version is >= version.
func (s *SchemaACKStore) ACKedDeviceCount(ctx context.Context, channelID string, version uint32) (int64, error) {
	pattern := fmt.Sprintf("schema_ack:%s:*", channelID)
	var count int64
	var cursor uint64
	for {
		keys, nextCursor, err := s.client.Scan(ctx, cursor, pattern, 100).Result()
		if err != nil {
			return 0, fmt.Errorf("SchemaACKStore.ACKedDeviceCount scan: %w", err)
		}
		for _, key := range keys {
			val, err := s.client.Get(ctx, key).Result()
			if errors.Is(err, redis.Nil) {
				continue
			}
			if err != nil {
				return 0, fmt.Errorf("SchemaACKStore.ACKedDeviceCount get %q: %w", key, err)
			}
			v, err := strconv.ParseUint(val, 10, 32)
			if err != nil {
				continue
			}
			if uint32(v) >= version {
				count++
			}
		}
		cursor = nextCursor
		if cursor == 0 {
			break
		}
	}
	return count, nil
}

// IsForceDeprecated reports whether channelID has an active force-deprecation marker.
// The marker is written by the device-registry service and expires after 48 hours.
// Returns false if the key does not exist or has expired.
func (s *SchemaACKStore) IsForceDeprecated(ctx context.Context, channelID string) (bool, error) {
	err := s.client.Get(ctx, schemaForceDeprecatedKey(channelID)).Err()
	if errors.Is(err, redis.Nil) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("SchemaACKStore.IsForceDeprecated: %w", err)
	}
	return true, nil
}

// SetStuck marks deviceID on channelID as stuck — it attempted to ingest during a
// force-deprecation window without updating its schema. The key expires after 48 hours,
// matching the force-deprecation window.
func (s *SchemaACKStore) SetStuck(ctx context.Context, channelID, deviceID string) error {
	key := schemaStuckKey(channelID, deviceID)
	if err := s.client.Set(ctx, key, "1", schemaStuckTTL).Err(); err != nil {
		return fmt.Errorf("SchemaACKStore.SetStuck: %w", err)
	}
	return nil
}

// scanCount counts the number of keys matching the given pattern using SCAN.
func (s *SchemaACKStore) scanCount(ctx context.Context, pattern string) (int64, error) {
	var count int64
	var cursor uint64
	for {
		keys, nextCursor, err := s.client.Scan(ctx, cursor, pattern, 100).Result()
		if err != nil {
			return 0, err
		}
		count += int64(len(keys))
		cursor = nextCursor
		if cursor == 0 {
			break
		}
	}
	return count, nil
}
