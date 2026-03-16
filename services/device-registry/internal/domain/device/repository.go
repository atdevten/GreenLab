package device

import (
	"context"

	"github.com/google/uuid"
)

// DeviceRepository defines persistence for Device.
type DeviceRepository interface {
	Create(ctx context.Context, d *Device) error
	GetByID(ctx context.Context, id uuid.UUID) (*Device, error)
	GetByAPIKey(ctx context.Context, apiKey string) (*Device, error)
	ListByWorkspace(ctx context.Context, workspaceID uuid.UUID, limit, offset int) ([]*Device, int64, error)
	Update(ctx context.Context, d *Device) error
	Delete(ctx context.Context, id uuid.UUID) error
}

// DeviceCacheRepository defines cache operations for device lookups.
type DeviceCacheRepository interface {
	SetDevice(ctx context.Context, device *Device) error
	GetDeviceByAPIKey(ctx context.Context, apiKey string) (*Device, error)
	DeleteDevice(ctx context.Context, deviceID, apiKey string) error
}
