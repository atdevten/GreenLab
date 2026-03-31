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

// ProvisionInput carries all data needed to atomically create a device + channel + fields.
type ProvisionInput struct {
	Device  ProvisionDeviceInput
	Channel ProvisionChannelInput
	Fields  []ProvisionFieldInput
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
	tx     TxRunner
	cache  device.DeviceCacheRepository
	logger *slog.Logger
}

// NewProvisionService constructs a ProvisionService.
func NewProvisionService(tx TxRunner, cache device.DeviceCacheRepository, logger *slog.Logger) *ProvisionService {
	return &ProvisionService{tx: tx, cache: cache, logger: logger}
}

// Provision creates a device, a channel linked to that device, and all supplied fields
// atomically inside a single Postgres transaction.
func (s *ProvisionService) Provision(ctx context.Context, in ProvisionInput) (*ProvisionResult, error) {
	wsID, err := uuid.Parse(in.Device.WorkspaceID)
	if err != nil {
		return nil, fmt.Errorf("Provision.ParseWorkspaceID: %w", err)
	}

	// Build domain objects before entering the transaction so validation errors
	// are returned before any DB round-trip.
	d, err := device.NewDevice(wsID, in.Device.Name, in.Device.Description)
	if err != nil {
		return nil, fmt.Errorf("Provision.NewDevice: %w", err)
	}

	ch, err := channel.NewChannel(wsID, in.Channel.Name, in.Channel.Description, channel.ChannelVisibility(in.Channel.Visibility))
	if err != nil {
		return nil, fmt.Errorf("Provision.NewChannel: %w", err)
	}
	ch.SetDevice(d.ID)

	// Fields are optional; a device+channel can be provisioned without any fields.
	fields := make([]*field.Field, 0, len(in.Fields))
	for i, fi := range in.Fields {
		f, err := field.NewField(ch.ID, fi.Name, fi.Label, fi.Unit, field.FieldType(fi.FieldType), fi.Position)
		if err != nil {
			return nil, fmt.Errorf("Provision.NewField[%d]: %w", i, err)
		}
		fields = append(fields, f)
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
