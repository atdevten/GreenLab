package application

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"

	"github.com/google/uuid"
	"github.com/greenlab/device-registry/internal/domain/channel"
	"github.com/greenlab/device-registry/internal/domain/device"
)

// DeviceService implements device management use-cases.
type DeviceService struct {
	repo   device.DeviceRepository
	tx     TxRunner
	cache  device.DeviceCacheRepository
	logger *slog.Logger
}

func NewDeviceService(repo device.DeviceRepository, tx TxRunner, cache device.DeviceCacheRepository, logger *slog.Logger) *DeviceService {
	return &DeviceService{repo: repo, tx: tx, cache: cache, logger: logger}
}

// LocationMetadata is the JSON shape stored in Device.Metadata for location data.
type LocationMetadata struct {
	Lat     *float64 `json:"lat,omitempty"`
	Lng     *float64 `json:"lng,omitempty"`
	Address string   `json:"address,omitempty"`
}

type CreateDeviceInput struct {
	WorkspaceID      string
	Name             string
	Description      string
	Lat              *float64
	Lng              *float64
	LocationAddress  string
	ChannelName      string
	ChannelVisibility string
}

func (s *DeviceService) CreateDevice(ctx context.Context, in CreateDeviceInput) (*device.Device, *channel.Channel, error) {
	wsID, err := uuid.Parse(in.WorkspaceID)
	if err != nil {
		return nil, nil, fmt.Errorf("CreateDevice.ParseWorkspaceID: %w", err)
	}
	d, err := device.NewDevice(wsID, in.Name, in.Description)
	if err != nil {
		return nil, nil, fmt.Errorf("CreateDevice.NewDevice: %w", err)
	}
	if (in.Lat != nil) != (in.Lng != nil) {
		return nil, nil, fmt.Errorf("CreateDevice: lat and lng must both be provided or both omitted")
	}
	if in.Lat != nil || in.Lng != nil || in.LocationAddress != "" {
		meta, err := json.Marshal(LocationMetadata{Lat: in.Lat, Lng: in.Lng, Address: in.LocationAddress})
		if err != nil {
			return nil, nil, fmt.Errorf("CreateDevice.MarshalLocation: %w", err)
		}
		d.Metadata = meta
	}
	chName := in.ChannelName
	if chName == "" {
		chName = "Channel 1"
	}
	chVisibility := channel.ChannelVisibilityPrivate
	if in.ChannelVisibility == string(channel.ChannelVisibilityPublic) {
		chVisibility = channel.ChannelVisibilityPublic
	}
	ch, err := channel.NewChannel(wsID, chName, "", chVisibility)
	if err != nil {
		return nil, nil, fmt.Errorf("CreateDevice.NewChannel: %w", err)
	}
	ch.SetDevice(d.ID)
	if err := s.tx.RunInTx(ctx, func(ctx context.Context, repos TxRepos) error {
		if err := repos.Devices.Create(ctx, d); err != nil {
			return fmt.Errorf("tx.Devices.Create: %w", err)
		}
		if err := repos.Channels.Create(ctx, ch); err != nil {
			return fmt.Errorf("tx.Channels.Create: %w", err)
		}
		return nil
	}); err != nil {
		return nil, nil, fmt.Errorf("CreateDevice.RunInTx: %w", err)
	}
	if err := s.cache.SetDevice(ctx, d); err != nil {
		s.logger.Error("failed to cache device", "device_id", d.ID, "error", err)
	}
	return d, ch, nil
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
