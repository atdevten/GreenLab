package redis

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"github.com/greenlab/device-registry/internal/domain/device"
)

const deviceCacheTTL = 5 * time.Minute

type DeviceCache struct {
	client *redis.Client
}

func NewDeviceCache(client *redis.Client) *DeviceCache {
	return &DeviceCache{client: client}
}

// deviceCacheEntry is the JSON representation stored in Redis.
// Kept here so the domain struct stays free of json tags.
type deviceCacheEntry struct {
	ID          uuid.UUID           `json:"id"`
	WorkspaceID uuid.UUID           `json:"workspace_id"`
	Name        string              `json:"name"`
	Description string              `json:"description"`
	APIKey      string              `json:"api_key"`
	Status      device.DeviceStatus `json:"status"`
	LastSeenAt  *time.Time          `json:"last_seen_at"`
	Metadata    []byte              `json:"metadata"`
	CreatedAt   time.Time           `json:"created_at"`
	UpdatedAt   time.Time           `json:"updated_at"`
}

func toEntry(d *device.Device) *deviceCacheEntry {
	return &deviceCacheEntry{
		ID:          d.ID,
		WorkspaceID: d.WorkspaceID,
		Name:        d.Name,
		Description: d.Description,
		APIKey:      d.APIKey,
		Status:      d.Status,
		LastSeenAt:  d.LastSeenAt,
		Metadata:    d.Metadata,
		CreatedAt:   d.CreatedAt,
		UpdatedAt:   d.UpdatedAt,
	}
}

func (e *deviceCacheEntry) toDomain() *device.Device {
	return &device.Device{
		ID:          e.ID,
		WorkspaceID: e.WorkspaceID,
		Name:        e.Name,
		Description: e.Description,
		APIKey:      e.APIKey,
		Status:      e.Status,
		LastSeenAt:  e.LastSeenAt,
		Metadata:    e.Metadata,
		CreatedAt:   e.CreatedAt,
		UpdatedAt:   e.UpdatedAt,
	}
}

func (c *DeviceCache) SetDevice(ctx context.Context, d *device.Device) error {
	b, err := json.Marshal(toEntry(d))
	if err != nil {
		return fmt.Errorf("DeviceCache.SetDevice marshal: %w", err)
	}
	keyByID := fmt.Sprintf("device:id:%s", d.ID.String())
	keyByKey := fmt.Sprintf("device:apikey:%s", d.APIKey)
	pipe := c.client.Pipeline()
	pipe.Set(ctx, keyByID, b, deviceCacheTTL)
	pipe.Set(ctx, keyByKey, b, deviceCacheTTL)
	cmds, err := pipe.Exec(ctx)
	if err != nil {
		return fmt.Errorf("DeviceCache.SetDevice pipeline: %w", err)
	}
	for _, cmd := range cmds {
		if cmd.Err() != nil {
			return fmt.Errorf("DeviceCache.SetDevice pipeline: %w", cmd.Err())
		}
	}
	return nil
}

func (c *DeviceCache) GetDeviceByAPIKey(ctx context.Context, apiKey string) (*device.Device, error) {
	key := fmt.Sprintf("device:apikey:%s", apiKey)
	b, err := c.client.Get(ctx, key).Bytes()
	if errors.Is(err, redis.Nil) {
		return nil, device.ErrCacheMiss
	}
	if err != nil {
		return nil, fmt.Errorf("DeviceCache.GetDeviceByAPIKey: %w", err)
	}
	var entry deviceCacheEntry
	if err := json.Unmarshal(b, &entry); err != nil {
		return nil, fmt.Errorf("DeviceCache.GetDeviceByAPIKey unmarshal: %w", err)
	}
	return entry.toDomain(), nil
}

func (c *DeviceCache) DeleteDevice(ctx context.Context, deviceID, apiKey string) error {
	if err := c.client.Del(ctx,
		fmt.Sprintf("device:id:%s", deviceID),
		fmt.Sprintf("device:apikey:%s", apiKey),
	).Err(); err != nil {
		return fmt.Errorf("DeviceCache.DeleteDevice: %w", err)
	}
	return nil
}

// IncrDeviceVersion atomically increments the version counter at
// "device_version:{deviceID}". Ingestion's cache compares this value
// on every cache hit to detect stale cached entries.
func (c *DeviceCache) IncrDeviceVersion(ctx context.Context, deviceID string) error {
	key := fmt.Sprintf("device_version:%s", deviceID)
	if err := c.client.Incr(ctx, key).Err(); err != nil {
		return fmt.Errorf("DeviceCache.IncrDeviceVersion: %w", err)
	}
	return nil
}
