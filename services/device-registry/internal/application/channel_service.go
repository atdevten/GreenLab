package application

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/greenlab/device-registry/internal/domain/channel"
)

type ChannelService struct {
	repo channel.ChannelRepository
}

func NewChannelService(repo channel.ChannelRepository) *ChannelService {
	return &ChannelService{repo: repo}
}

type CreateChannelInput struct {
	WorkspaceID string
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
	if err := s.repo.Create(ctx, ch); err != nil {
		return nil, fmt.Errorf("CreateChannel.repo.Create: %w", err)
	}
	return ch, nil
}

func (s *ChannelService) GetChannel(ctx context.Context, id string) (*channel.Channel, error) {
	uid, err := uuid.Parse(id)
	if err != nil {
		return nil, fmt.Errorf("GetChannel.ParseID: %w", err)
	}
	return s.repo.GetByID(ctx, uid)
}

func (s *ChannelService) ListChannels(ctx context.Context, workspaceID string, limit, offset int) ([]*channel.Channel, int64, error) {
	wsID, err := uuid.Parse(workspaceID)
	if err != nil {
		return nil, 0, fmt.Errorf("ListChannels.ParseWorkspaceID: %w", err)
	}
	return s.repo.ListByWorkspace(ctx, wsID, limit, offset)
}

func (s *ChannelService) ListChannelsByDevice(ctx context.Context, deviceID string, limit, offset int) ([]*channel.Channel, int64, error) {
	devID, err := uuid.Parse(deviceID)
	if err != nil {
		return nil, 0, fmt.Errorf("ListChannelsByDevice.ParseDeviceID: %w", err)
	}
	return s.repo.ListByDevice(ctx, devID, limit, offset)
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
		return nil, fmt.Errorf("UpdateChannel.repo.Update: %w", err)
	}
	return ch, nil
}

func (s *ChannelService) DeleteChannel(ctx context.Context, id string) error {
	uid, err := uuid.Parse(id)
	if err != nil {
		return fmt.Errorf("DeleteChannel.ParseID: %w", err)
	}
	return s.repo.Delete(ctx, uid)
}
