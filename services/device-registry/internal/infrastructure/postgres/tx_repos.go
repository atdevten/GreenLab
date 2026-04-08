package postgres

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/greenlab/device-registry/internal/domain/channel"
	"github.com/greenlab/device-registry/internal/domain/device"
	"github.com/greenlab/device-registry/internal/domain/field"
)

// txDeviceRepo is a write-only device repo bound to a transaction.
// Only Create is required for the provision flow; the other methods panic if called.
type txDeviceRepo struct{ tx *sqlx.Tx }

func newTxDeviceRepo(tx *sqlx.Tx) *txDeviceRepo { return &txDeviceRepo{tx: tx} }

func (r *txDeviceRepo) Create(ctx context.Context, d *device.Device) error {
	_, err := r.tx.NamedExecContext(ctx, `
		INSERT INTO devices (id, workspace_id, name, description, api_key, status, metadata)
		VALUES (:id, :workspace_id, :name, :description, :api_key, :status, :metadata)`, toDeviceRow(d))
	if err != nil {
		return fmt.Errorf("txDeviceRepo.Create: %w", err)
	}
	return nil
}

func (r *txDeviceRepo) GetByID(_ context.Context, _ uuid.UUID) (*device.Device, error) {
	panic("txDeviceRepo.GetByID not implemented for transactional use")
}
func (r *txDeviceRepo) GetByAPIKey(_ context.Context, _ string) (*device.Device, error) {
	panic("txDeviceRepo.GetByAPIKey not implemented for transactional use")
}
func (r *txDeviceRepo) ListByWorkspace(_ context.Context, _ uuid.UUID, _, _ int) ([]*device.Device, int64, error) {
	panic("txDeviceRepo.ListByWorkspace not implemented for transactional use")
}
func (r *txDeviceRepo) Update(_ context.Context, _ *device.Device) error {
	panic("txDeviceRepo.Update not implemented for transactional use")
}
func (r *txDeviceRepo) Delete(_ context.Context, _ uuid.UUID) error {
	panic("txDeviceRepo.Delete not implemented for transactional use")
}

// txChannelRepo is a write-only channel repo bound to a transaction.
type txChannelRepo struct{ tx *sqlx.Tx }

func newTxChannelRepo(tx *sqlx.Tx) *txChannelRepo { return &txChannelRepo{tx: tx} }

func (r *txChannelRepo) Create(ctx context.Context, ch *channel.Channel) error {
	row := toChannelRow(ch)
	err := r.tx.QueryRowContext(ctx, `
		INSERT INTO channels (id, workspace_id, device_id, name, description, visibility, tags)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING short_id`,
		row.ID, row.WorkspaceID, row.DeviceID, row.Name, row.Description, row.Visibility, row.Tags,
	).Scan(&ch.ShortID)
	if err != nil {
		return fmt.Errorf("txChannelRepo.Create: %w", err)
	}
	return nil
}

func (r *txChannelRepo) GetByID(_ context.Context, _ uuid.UUID) (*channel.Channel, error) {
	panic("txChannelRepo.GetByID not implemented for transactional use")
}
func (r *txChannelRepo) ListByWorkspace(_ context.Context, _ uuid.UUID, _, _ int) ([]*channel.Channel, int64, error) {
	panic("txChannelRepo.ListByWorkspace not implemented for transactional use")
}
func (r *txChannelRepo) ListByDevice(_ context.Context, _ uuid.UUID, _, _ int) ([]*channel.Channel, int64, error) {
	panic("txChannelRepo.ListByDevice not implemented for transactional use")
}
func (r *txChannelRepo) Update(ctx context.Context, ch *channel.Channel) error {
	_, err := r.tx.ExecContext(ctx,
		`UPDATE channels SET device_id=$1, updated_at=NOW() WHERE id=$2`,
		ch.DeviceID, ch.ID)
	if err != nil {
		return fmt.Errorf("txChannelRepo.Update: %w", err)
	}
	return nil
}
func (r *txChannelRepo) Delete(_ context.Context, _ uuid.UUID) error {
	panic("txChannelRepo.Delete not implemented for transactional use")
}

// txFieldRepo is a write-only field repo bound to a transaction.
type txFieldRepo struct{ tx *sqlx.Tx }

func newTxFieldRepo(tx *sqlx.Tx) *txFieldRepo { return &txFieldRepo{tx: tx} }

func (r *txFieldRepo) Create(ctx context.Context, f *field.Field) error {
	_, err := r.tx.NamedExecContext(ctx, `
		INSERT INTO fields (id, channel_id, name, label, unit, field_type, position, description)
		VALUES (:id, :channel_id, :name, :label, :unit, :field_type, :position, :description)`, toFieldRow(f))
	if err != nil {
		return fmt.Errorf("txFieldRepo.Create: %w", err)
	}
	return nil
}

func (r *txFieldRepo) GetByID(_ context.Context, _ uuid.UUID) (*field.Field, error) {
	panic("txFieldRepo.GetByID not implemented for transactional use")
}
func (r *txFieldRepo) ListByChannel(_ context.Context, _ uuid.UUID) ([]*field.Field, error) {
	panic("txFieldRepo.ListByChannel not implemented for transactional use")
}
func (r *txFieldRepo) Update(_ context.Context, _ *field.Field) error {
	panic("txFieldRepo.Update not implemented for transactional use")
}
func (r *txFieldRepo) Delete(_ context.Context, _ uuid.UUID) error {
	panic("txFieldRepo.Delete not implemented for transactional use")
}
