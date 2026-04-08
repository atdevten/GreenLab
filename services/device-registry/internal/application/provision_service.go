package application

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/google/uuid"
	"github.com/greenlab/device-registry/internal/domain/channel"
	"github.com/greenlab/device-registry/internal/domain/device"
	"github.com/greenlab/device-registry/internal/domain/field"
)

// TxRunner executes fn inside a single database transaction.
// The implementation calls fn with repo factories scoped to the transaction.
// On any error returned by fn the transaction is rolled back; otherwise it is committed.
type TxRunner interface {
	RunInTx(ctx context.Context, fn func(ctx context.Context, tx TxRepos) error) error
}

// TxRepos provides repository instances bound to the active transaction.
type TxRepos struct {
	Devices  device.DeviceRepository
	Channels channel.ChannelRepository
	Fields   field.FieldRepository
}

// channelLookup is the read-only channel accessor needed by the provision flow
// when linking a device to an existing channel.
type channelLookup interface {
	GetByID(ctx context.Context, id uuid.UUID) (*channel.Channel, error)
}

// ProvisionInput carries all data needed to atomically create a device + channel + fields.
type ProvisionInput struct {
	Device           ProvisionDeviceInput
	Channel          ProvisionChannelInput
	ExistingChannelID string // when non-empty, link to this channel instead of creating one
	Fields           []ProvisionFieldInput
}

// ProvisionDeviceInput is the device portion of a provision request.
type ProvisionDeviceInput struct {
	WorkspaceID string
	Name        string
	Description string
}

// ProvisionChannelInput is the channel portion of a provision request.
type ProvisionChannelInput struct {
	Name        string
	Description string
	Visibility  string
}

// ProvisionFieldInput is one field within a provision request.
type ProvisionFieldInput struct {
	Name      string
	Label     string
	Unit      string
	FieldType string
	Position  int
}

// ProvisionResult is the fully created graph returned to the caller.
type ProvisionResult struct {
	Device  *device.Device
	Channel *channel.Channel
	Fields  []*field.Field
}

// ProvisionService orchestrates atomic device + channel + field creation.
type ProvisionService struct {
	tx       TxRunner
	channels channelLookup
	cache    device.DeviceCacheRepository
	logger   *slog.Logger
}

// NewProvisionService constructs a ProvisionService.
func NewProvisionService(tx TxRunner, channels channelLookup, cache device.DeviceCacheRepository, logger *slog.Logger) *ProvisionService {
	return &ProvisionService{tx: tx, channels: channels, cache: cache, logger: logger}
}

// Provision atomically creates a device, links it to a channel (new or existing), and
// creates the supplied fields. When ExistingChannelID is set the identified channel is
// fetched, its device_id is updated inside the transaction, and no new channel row is
// inserted. When ExistingChannelID is empty a new channel is created (original behaviour).
func (s *ProvisionService) Provision(ctx context.Context, in ProvisionInput) (*ProvisionResult, error) {
	wsID, err := uuid.Parse(in.Device.WorkspaceID)
	if err != nil {
		return nil, fmt.Errorf("Provision.ParseWorkspaceID: %w", err)
	}

	// Build the device domain object before entering the transaction so validation
	// errors are returned without any DB round-trip.
	d, err := device.NewDevice(wsID, in.Device.Name, in.Device.Description)
	if err != nil {
		return nil, fmt.Errorf("Provision.NewDevice: %w", err)
	}

	if in.ExistingChannelID != "" {
		return s.provisionWithExistingChannel(ctx, in, d)
	}
	return s.provisionWithNewChannel(ctx, in, wsID, d)
}

// provisionWithNewChannel is the original path: create device + new channel + fields atomically.
func (s *ProvisionService) provisionWithNewChannel(ctx context.Context, in ProvisionInput, wsID uuid.UUID, d *device.Device) (*ProvisionResult, error) {
	ch, err := channel.NewChannel(wsID, in.Channel.Name, in.Channel.Description, channel.ChannelVisibility(in.Channel.Visibility))
	if err != nil {
		return nil, fmt.Errorf("Provision.NewChannel: %w", err)
	}
	ch.SetDevice(d.ID)

	fields, err := buildFields(ch.ID, in.Fields)
	if err != nil {
		return nil, err
	}

	if err := s.tx.RunInTx(ctx, func(ctx context.Context, repos TxRepos) error {
		if err := repos.Devices.Create(ctx, d); err != nil {
			return fmt.Errorf("tx.Devices.Create: %w", err)
		}
		if err := repos.Channels.Create(ctx, ch); err != nil {
			return fmt.Errorf("tx.Channels.Create: %w", err)
		}
		for _, f := range fields {
			if err := repos.Fields.Create(ctx, f); err != nil {
				return fmt.Errorf("tx.Fields.Create: %w", err)
			}
		}
		return nil
	}); err != nil {
		return nil, fmt.Errorf("Provision.RunInTx: %w", err)
	}

	// Best-effort cache warm; never fail the provision on cache errors.
	if err := s.cache.SetDevice(ctx, d); err != nil {
		s.logger.Error("provision: failed to cache device", "device_id", d.ID, "error", err)
	}

	return &ProvisionResult{Device: d, Channel: ch, Fields: fields}, nil
}

// provisionWithExistingChannel links the new device to a channel that already exists.
// The channel's device_id is updated transactionally.
func (s *ProvisionService) provisionWithExistingChannel(ctx context.Context, in ProvisionInput, d *device.Device) (*ProvisionResult, error) {
	chID, err := uuid.Parse(in.ExistingChannelID)
	if err != nil {
		return nil, fmt.Errorf("Provision.ParseChannelID: %w", err)
	}

	// Fetch and validate the channel existence before entering the transaction.
	ch, err := s.channels.GetByID(ctx, chID)
	if err != nil {
		return nil, fmt.Errorf("Provision.GetChannel: %w", err)
	}

	fields, err := buildFields(ch.ID, in.Fields)
	if err != nil {
		return nil, err
	}

	if err := s.tx.RunInTx(ctx, func(ctx context.Context, repos TxRepos) error {
		if err := repos.Devices.Create(ctx, d); err != nil {
			return fmt.Errorf("tx.Devices.Create: %w", err)
		}
		ch.SetDevice(d.ID)
		if err := repos.Channels.Update(ctx, ch); err != nil {
			return fmt.Errorf("tx.Channels.Update: %w", err)
		}
		for _, f := range fields {
			if err := repos.Fields.Create(ctx, f); err != nil {
				return fmt.Errorf("tx.Fields.Create: %w", err)
			}
		}
		return nil
	}); err != nil {
		return nil, fmt.Errorf("Provision.RunInTx: %w", err)
	}

	// Best-effort cache warm; never fail the provision on cache errors.
	if err := s.cache.SetDevice(ctx, d); err != nil {
		s.logger.Error("provision: failed to cache device", "device_id", d.ID, "error", err)
	}

	return &ProvisionResult{Device: d, Channel: ch, Fields: fields}, nil
}

// buildFields constructs validated field domain objects for the given channel.
func buildFields(channelID uuid.UUID, inputs []ProvisionFieldInput) ([]*field.Field, error) {
	fields := make([]*field.Field, 0, len(inputs))
	for i, fi := range inputs {
		f, err := field.NewField(channelID, fi.Name, fi.Label, fi.Unit, field.FieldType(fi.FieldType), fi.Position)
		if err != nil {
			return nil, fmt.Errorf("Provision.NewField[%d]: %w", i, err)
		}
		fields = append(fields, f)
	}
	return fields, nil
}
