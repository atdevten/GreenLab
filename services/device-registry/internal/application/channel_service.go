package application

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/google/uuid"
	"github.com/greenlab/device-registry/internal/domain/channel"
)

type ChannelService struct {
	repo   channel.ChannelRepository
	logger *slog.Logger
}

func NewChannelService(repo channel.ChannelRepository, logger *slog.Logger) *ChannelService {
	return &ChannelService{repo: repo, logger: logger}
}

type CreateChannelInput struct {
	WorkspaceID string
	DeviceID    *string // optional
	Name        string
	Description string
	Visibility  string
}

func (s *ChannelService) CreateChannel(ctx context.Context, in CreateChannelInput) (*channel.Channel, error) {
	wsID, err := uuid.Parse(in.WorkspaceID)
	if err != nil {
		return nil, fmt.Errorf("CreateChannel.ParseWorkspaceID: %w", err)
	}
	ch, err := channel.NewChannel(wsID, in.Name, in.Description, channel.ChannelVisibility(in.Visibility))
	if err != nil {
		return nil, fmt.Errorf("CreateChannel.NewChannel: %w", err)
	}
	if in.DeviceID != nil {
		devID, err := uuid.Parse(*in.DeviceID)
		if err != nil {
			return nil, fmt.Errorf("CreateChannel.ParseDeviceID: %w", err)
		}
		ch.DeviceID = &devID
	}
	if err := s.repo.Create(ctx, ch); err != nil {
		s.logger.ErrorContext(ctx, "CreateChannel: repo.Create failed", "error", err)
		return nil, fmt.Errorf("CreateChannel.repo.Create: %w", err)
	}
	return ch, nil
}

func (s *ChannelService) GetChannel(ctx context.Context, id string) (*channel.Channel, error) {
	uid, err := uuid.Parse(id)
	if err != nil {
		return nil, fmt.Errorf("GetChannel.ParseID: %w", err)
	}
	ch, err := s.repo.GetByID(ctx, uid)
	if err != nil {
		s.logger.ErrorContext(ctx, "GetChannel: repo.GetByID failed", "error", err, "id", id)
		return nil, err
	}
	return ch, nil
}

func (s *ChannelService) ListChannels(ctx context.Context, workspaceID string, limit, offset int) ([]*channel.Channel, int64, error) {
	wsID, err := uuid.Parse(workspaceID)
	if err != nil {
		return nil, 0, fmt.Errorf("ListChannels.ParseWorkspaceID: %w", err)
	}
	channels, total, err := s.repo.ListByWorkspace(ctx, wsID, limit, offset)
	if err != nil {
		s.logger.ErrorContext(ctx, "ListChannels: repo.ListByWorkspace failed", "error", err, "workspace_id", workspaceID)
		return nil, 0, err
	}
	return channels, total, nil
}

func (s *ChannelService) ListChannelsByDevice(ctx context.Context, deviceID string, limit, offset int) ([]*channel.Channel, int64, error) {
	devID, err := uuid.Parse(deviceID)
	if err != nil {
		return nil, 0, fmt.Errorf("ListChannelsByDevice.ParseDeviceID: %w", err)
	}
	channels, total, err := s.repo.ListByDevice(ctx, devID, limit, offset)
	if err != nil {
		s.logger.ErrorContext(ctx, "ListChannelsByDevice: repo.ListByDevice failed", "error", err, "device_id", deviceID)
		return nil, 0, err
	}
	return channels, total, nil
}

type UpdateChannelInput struct {
	Name        string
	Description string
	Visibility  string
}

func (s *ChannelService) UpdateChannel(ctx context.Context, id string, in UpdateChannelInput) (*channel.Channel, error) {
	uid, err := uuid.Parse(id)
	if err != nil {
		return nil, fmt.Errorf("UpdateChannel.ParseID: %w", err)
	}
	ch, err := s.repo.GetByID(ctx, uid)
	if err != nil {
		s.logger.ErrorContext(ctx, "UpdateChannel: repo.GetByID failed", "error", err, "id", id)
		return nil, fmt.Errorf("UpdateChannel.GetByID: %w", err)
	}
	if in.Name != "" {
		if err := ch.SetName(in.Name); err != nil {
			return nil, fmt.Errorf("UpdateChannel.SetName: %w", err)
		}
	}
	if in.Description != "" {
		ch.Description = in.Description
	}
	if in.Visibility != "" {
		if err := ch.SetVisibility(channel.ChannelVisibility(in.Visibility)); err != nil {
			return nil, fmt.Errorf("UpdateChannel.SetVisibility: %w", err)
		}
	}
	if err := s.repo.Update(ctx, ch); err != nil {
		s.logger.ErrorContext(ctx, "UpdateChannel: repo.Update failed", "error", err, "id", id)
		return nil, fmt.Errorf("UpdateChannel.repo.Update: %w", err)
	}
	return ch, nil
}

func (s *ChannelService) DeleteChannel(ctx context.Context, id string) error {
	uid, err := uuid.Parse(id)
	if err != nil {
		return fmt.Errorf("DeleteChannel.ParseID: %w", err)
	}
	if err := s.repo.Delete(ctx, uid); err != nil {
		s.logger.ErrorContext(ctx, "DeleteChannel: repo.Delete failed", "error", err, "id", id)
		return err
	}
	return nil
}
