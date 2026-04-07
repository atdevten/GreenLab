package device

import (
	"crypto/rand"
	"encoding/hex"
	"strings"
	"time"

	"github.com/google/uuid"
)

// DeviceStatus represents the lifecycle state of a device.
type DeviceStatus string

const (
	DeviceStatusActive   DeviceStatus = "active"
	DeviceStatusInactive DeviceStatus = "inactive"
	DeviceStatusBlocked  DeviceStatus = "blocked"
)

// Device is the core domain entity.
type Device struct {
	ID          uuid.UUID
	WorkspaceID uuid.UUID
	Name        string
	Description string
	APIKey      string
	Status      DeviceStatus
	LastSeenAt  *time.Time
	Metadata    []byte // JSONB
	CreatedAt   time.Time
	UpdatedAt   time.Time
	DeletedAt   *time.Time
}

// SetName validates and sets the device name.
func (d *Device) SetName(name string) error {
	if strings.TrimSpace(name) == "" {
		return ErrInvalidName
	}
	d.Name = strings.TrimSpace(name)
	d.UpdatedAt = time.Now().UTC()
	return nil
}

// NewDevice creates a new Device with a generated API key.
func NewDevice(workspaceID uuid.UUID, name, description string) (*Device, error) {
	if strings.TrimSpace(name) == "" {
		return nil, ErrInvalidName
	}
	key, err := generateAPIKey()
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	return &Device{
		ID:          uuid.New(),
		WorkspaceID: workspaceID,
		Name:        name,
		Description: description,
		APIKey:      key,
		Status:      DeviceStatusActive,
		CreatedAt:   now,
		UpdatedAt:   now,
	}, nil
}

// RotateAPIKey generates a new API key for the device.
func (d *Device) RotateAPIKey() error {
	key, err := generateAPIKey()
	if err != nil {
		return err
	}
	d.APIKey = key
	d.UpdatedAt = time.Now().UTC()
	return nil
}

// IsActive returns true if the device can ingest data.
func (d *Device) IsActive() bool {
	return d.Status == DeviceStatusActive
}

// SetStatus validates and sets the device status.
func (d *Device) SetStatus(status DeviceStatus) error {
	switch status {
	case DeviceStatusActive, DeviceStatusInactive, DeviceStatusBlocked:
		d.Status = status
		d.UpdatedAt = time.Now().UTC()
		return nil
	default:
		return ErrInvalidStatus
	}
}

// SoftDelete marks the device as deleted by setting DeletedAt to now.
func (d *Device) SoftDelete() error {
	if d.DeletedAt != nil {
		return ErrDeviceAlreadyDeleted
	}
	now := time.Now().UTC()
	d.DeletedAt = &now
	d.UpdatedAt = now
	return nil
}

// IsDeleted returns true if the device has been soft-deleted.
func (d *Device) IsDeleted() bool {
	return d.DeletedAt != nil
}

func generateAPIKey() (string, error) {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return "g_" + hex.EncodeToString(b), nil
}
