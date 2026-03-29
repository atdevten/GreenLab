package application

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/google/uuid"
	"github.com/greenlab/device-registry/internal/domain/device"
)

// DeviceService implements device management use-cases.
type DeviceService struct {
	repo   device.DeviceRepository
	cache  device.DeviceCacheRepository
	logger *slog.Logger
}

func NewDeviceService(repo device.DeviceRepository, cache device.DeviceCacheRepository, logger *slog.Logger) *DeviceService {
	return &DeviceService{repo: repo, cache: cache, logger: logger}
}

type CreateDeviceInput struct {
	WorkspaceID string
	Name        string
	Description string
}

func (s *DeviceService) CreateDevice(ctx context.Context, in CreateDeviceInput) (*device.Device, error) {
	wsID, err := uuid.Parse(in.WorkspaceID)
	if err != nil {
		return nil, fmt.Errorf("CreateDevice.ParseWorkspaceID: %w", err)
	}
	d, err := device.NewDevice(wsID, in.Name, in.Description)
	if err != nil {
		return nil, fmt.Errorf("CreateDevice.NewDevice: %w", err)
	}
	if err := s.repo.Create(ctx, d); err != nil {
		return nil, fmt.Errorf("CreateDevice.repo.Create: %w", err)
	}
	if err := s.cache.SetDevice(ctx, d); err != nil {
		s.logger.Error("failed to cache device", "device_id", d.ID, "error", err)
	}
	return d, nil
}

func (s *DeviceService) GetDevice(ctx context.Context, id string) (*device.Device, error) {
	uid, err := uuid.Parse(id)
	if err != nil {
		return nil, fmt.Errorf("GetDevice.ParseID: %w", err)
	}
	return s.repo.GetByID(ctx, uid)
}

func (s *DeviceService) ListDevices(ctx context.Context, workspaceID string, limit, offset int) ([]*device.Device, int64, error) {
	wsID, err := uuid.Parse(workspaceID)
	if err != nil {
		return nil, 0, fmt.Errorf("ListDevices.ParseWorkspaceID: %w", err)
	}
	return s.repo.ListByWorkspace(ctx, wsID, limit, offset)
}

type UpdateDeviceInput struct {
	Name        string
	Description string
	Status      string
}

func (s *DeviceService) UpdateDevice(ctx context.Context, id string, in UpdateDeviceInput) (*device.Device, error) {
	uid, err := uuid.Parse(id)
	if err != nil {
		return nil, fmt.Errorf("UpdateDevice.ParseID: %w", err)
	}
	d, err := s.repo.GetByID(ctx, uid)
	if err != nil {
		return nil, fmt.Errorf("UpdateDevice.GetByID: %w", err)
	}
	if in.Name != "" {
		if err := d.SetName(in.Name); err != nil {
			return nil, fmt.Errorf("UpdateDevice.SetName: %w", err)
		}
	}
	if in.Description != "" {
		d.Description = in.Description
	}
	if in.Status != "" {
		if err := d.SetStatus(device.DeviceStatus(in.Status)); err != nil {
			return nil, fmt.Errorf("UpdateDevice.SetStatus: %w", err)
		}
	}
	if err := s.repo.Update(ctx, d); err != nil {
		return nil, fmt.Errorf("UpdateDevice.repo.Update: %w", err)
	}
	if err := s.cache.SetDevice(ctx, d); err != nil {
		s.logger.Error("failed to cache device", "device_id", d.ID, "error", err)
	}
	return d, nil
}

func (s *DeviceService) RotateAPIKey(ctx context.Context, id string) (*device.Device, error) {
	uid, err := uuid.Parse(id)
	if err != nil {
		return nil, fmt.Errorf("RotateAPIKey.ParseID: %w", err)
	}
	d, err := s.repo.GetByID(ctx, uid)
	if err != nil {
		return nil, fmt.Errorf("RotateAPIKey.GetByID: %w", err)
	}
	if err := d.RotateAPIKey(); err != nil {
		return nil, fmt.Errorf("RotateAPIKey.RotateAPIKey: %w", err)
	}
	if err := s.repo.Update(ctx, d); err != nil {
		return nil, fmt.Errorf("RotateAPIKey.repo.Update: %w", err)
	}
	if err := s.cache.SetDevice(ctx, d); err != nil {
		s.logger.Error("failed to cache device", "device_id", d.ID, "error", err)
	}
	if err := s.cache.IncrDeviceVersion(ctx, d.ID.String()); err != nil {
		s.logger.Error("failed to increment device version", "device_id", d.ID, "error", err)
	}
	return d, nil
}

func (s *DeviceService) DeleteDevice(ctx context.Context, id string) error {
	uid, err := uuid.Parse(id)
	if err != nil {
		return fmt.Errorf("DeleteDevice.ParseID: %w", err)
	}
	d, err := s.repo.GetByID(ctx, uid)
	if err != nil {
		return fmt.Errorf("DeleteDevice.GetByID: %w", err)
	}
	if err := d.SoftDelete(); err != nil {
		return fmt.Errorf("DeleteDevice.SoftDelete: %w", err)
	}
	if err := s.repo.Delete(ctx, uid); err != nil {
		return fmt.Errorf("DeleteDevice.repo.Delete: %w", err)
	}
	if err := s.cache.DeleteDevice(ctx, d.ID.String(), d.APIKey); err != nil {
		s.logger.Error("failed to delete device from cache", "device_id", d.ID, "error", err)
	}
	if err := s.cache.IncrDeviceVersion(ctx, d.ID.String()); err != nil {
		s.logger.Error("failed to increment device version", "device_id", d.ID, "error", err)
	}
	return nil
}

// ValidateAPIKey looks up a device by API key, using cache first.
func (s *DeviceService) ValidateAPIKey(ctx context.Context, apiKey string) (*device.Device, error) {
	if d, err := s.cache.GetDeviceByAPIKey(ctx, apiKey); err == nil {
		return d, nil
	} else if !errors.Is(err, device.ErrCacheMiss) {
		s.logger.Error("cache lookup failed", "error", err)
	}
	d, err := s.repo.GetByAPIKey(ctx, apiKey)
	if err != nil {
		return nil, err
	}
	if err := s.cache.SetDevice(ctx, d); err != nil {
		s.logger.Error("failed to cache device", "device_id", d.ID, "error", err)
	}
	return d, nil
}
