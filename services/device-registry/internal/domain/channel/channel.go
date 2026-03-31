package channel

import (
	"strings"
	"time"

	"github.com/google/uuid"
)

// ChannelVisibility controls whether a channel is public or private.
type ChannelVisibility string

const (
	ChannelVisibilityPublic  ChannelVisibility = "public"
	ChannelVisibilityPrivate ChannelVisibility = "private"
)

// Channel aggregates fields for IoT data collection.
type Channel struct {
	ID          uuid.UUID
	WorkspaceID uuid.UUID
	DeviceID    *uuid.UUID
	Name        string
	Description string
	Visibility  ChannelVisibility
	Tags        []byte // JSONB string array
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// SetDevice links a device to this channel.
// Use this instead of direct field assignment so any future guard on DeviceID
// has a single place to enforce it.
func (ch *Channel) SetDevice(deviceID uuid.UUID) {
	ch.DeviceID = &deviceID
	ch.UpdatedAt = time.Now().UTC()
}

// SetName validates and sets the channel name.
func (ch *Channel) SetName(name string) error {
	if strings.TrimSpace(name) == "" {
		return ErrInvalidName
	}
	ch.Name = strings.TrimSpace(name)
	ch.UpdatedAt = time.Now().UTC()
	return nil
}

// SetVisibility validates and sets the channel visibility.
func (ch *Channel) SetVisibility(v ChannelVisibility) error {
	if v != ChannelVisibilityPublic && v != ChannelVisibilityPrivate {
		return ErrInvalidVisibility
	}
	ch.Visibility = v
	ch.UpdatedAt = time.Now().UTC()
	return nil
}

// NewChannel creates a new Channel with validation.
func NewChannel(workspaceID uuid.UUID, name, description string, visibility ChannelVisibility) (*Channel, error) {
	if strings.TrimSpace(name) == "" {
		return nil, ErrInvalidName
	}
	if visibility == "" {
		visibility = ChannelVisibilityPrivate
	}
	if visibility != ChannelVisibilityPublic && visibility != ChannelVisibilityPrivate {
		return nil, ErrInvalidVisibility
	}
	now := time.Now().UTC()
	return &Channel{
		ID:          uuid.New(),
		WorkspaceID: workspaceID,
		Name:        name,
		Description: description,
		Visibility:  visibility,
		Tags:        []byte("[]"),
		CreatedAt:   now,
		UpdatedAt:   now,
	}, nil
}
