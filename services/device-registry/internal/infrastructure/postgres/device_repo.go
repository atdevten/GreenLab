package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/greenlab/device-registry/internal/domain/device"
)

type deviceRow struct {
	ID          uuid.UUID           `db:"id"`
	WorkspaceID uuid.UUID           `db:"workspace_id"`
	Name        string              `db:"name"`
	Description string              `db:"description"`
	APIKey      string              `db:"api_key"`
	Status      device.DeviceStatus `db:"status"`
	LastSeenAt  *time.Time          `db:"last_seen_at"`
	Metadata    []byte              `db:"metadata"`
	CreatedAt   time.Time           `db:"created_at"`
	UpdatedAt   time.Time           `db:"updated_at"`
	DeletedAt   *time.Time          `db:"deleted_at"`
}

func (r *deviceRow) toDomain() *device.Device {
	return &device.Device{
		ID:          r.ID,
		WorkspaceID: r.WorkspaceID,
		Name:        r.Name,
		Description: r.Description,
		APIKey:      r.APIKey,
		Status:      r.Status,
		LastSeenAt:  r.LastSeenAt,
		Metadata:    r.Metadata,
		CreatedAt:   r.CreatedAt,
		UpdatedAt:   r.UpdatedAt,
		DeletedAt:   r.DeletedAt,
	}
}

func toDeviceRow(d *device.Device) *deviceRow {
	return &deviceRow{
		ID:          d.ID,
		WorkspaceID: d.WorkspaceID,
		Name:        d.Name,
		Description: d.Description,
		APIKey:      d.APIKey,
		Status:      d.Status,
		LastSeenAt:  d.LastSeenAt,
		Metadata:    d.Metadata,
		CreatedAt:   d.CreatedAt,
		UpdatedAt:   d.UpdatedAt,
		DeletedAt:   d.DeletedAt,
	}
}

type DeviceRepo struct{ db *sqlx.DB }

func NewDeviceRepo(db *sqlx.DB) *DeviceRepo { return &DeviceRepo{db: db} }

func (r *DeviceRepo) Create(ctx context.Context, d *device.Device) error {
	_, err := r.db.NamedExecContext(ctx, `
		INSERT INTO devices (id, workspace_id, name, description, api_key, status, metadata)
		VALUES (:id, :workspace_id, :name, :description, :api_key, :status, :metadata)`, toDeviceRow(d))
	if err != nil {
		return fmt.Errorf("DeviceRepo.Create: %w", err)
	}
	return nil
}

func (r *DeviceRepo) GetByID(ctx context.Context, id uuid.UUID) (*device.Device, error) {
	var row deviceRow
	err := r.db.GetContext(ctx, &row,
		`SELECT id, workspace_id, name, description, api_key, status, last_seen_at, metadata, created_at, updated_at, deleted_at FROM devices WHERE id=$1 AND deleted_at IS NULL`, id)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("DeviceRepo.GetByID: %w", device.ErrDeviceNotFound)
	}
	if err != nil {
		return nil, fmt.Errorf("DeviceRepo.GetByID: %w", err)
	}
	return row.toDomain(), nil
}

func (r *DeviceRepo) GetByAPIKey(ctx context.Context, apiKey string) (*device.Device, error) {
	var row deviceRow
	err := r.db.GetContext(ctx, &row,
		`SELECT id, workspace_id, name, description, api_key, status, last_seen_at, metadata, created_at, updated_at, deleted_at FROM devices WHERE api_key=$1 AND status=$2 AND deleted_at IS NULL`, apiKey, string(device.DeviceStatusActive))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("DeviceRepo.GetByAPIKey: %w", device.ErrDeviceNotFound)
	}
	if err != nil {
		return nil, fmt.Errorf("DeviceRepo.GetByAPIKey: %w", err)
	}
	return row.toDomain(), nil
}

type deviceListRow struct {
	deviceRow
	TotalCount int64 `db:"total_count"`
}

func (r *DeviceRepo) ListByWorkspace(ctx context.Context, workspaceID uuid.UUID, limit, offset int) ([]*device.Device, int64, error) {
	var rows []deviceListRow
	err := r.db.SelectContext(ctx, &rows, `
		SELECT id, workspace_id, name, description, api_key, status, last_seen_at, metadata, created_at, updated_at, deleted_at,
		       COUNT(*) OVER() AS total_count
		FROM devices WHERE workspace_id=$1 AND deleted_at IS NULL ORDER BY created_at DESC LIMIT $2 OFFSET $3`,
		workspaceID, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("DeviceRepo.ListByWorkspace: %w", err)
	}
	var total int64
	devices := make([]*device.Device, len(rows))
	for i := range rows {
		if i == 0 {
			total = rows[i].TotalCount
		}
		devices[i] = rows[i].deviceRow.toDomain()
	}
	return devices, total, nil
}

func (r *DeviceRepo) Update(ctx context.Context, d *device.Device) error {
	_, err := r.db.ExecContext(ctx, `
		UPDATE devices SET name=$1, description=$2, api_key=$3, status=$4, updated_at=NOW() WHERE id=$5`,
		d.Name, d.Description, d.APIKey, d.Status, d.ID)
	if err != nil {
		return fmt.Errorf("DeviceRepo.Update: %w", err)
	}
	return nil
}

func (r *DeviceRepo) Delete(ctx context.Context, id uuid.UUID) error {
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return fmt.Errorf("DeviceRepo.Delete: begin tx: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck

	res, err := tx.ExecContext(ctx,
		`UPDATE devices SET deleted_at=NOW(), updated_at=NOW() WHERE id=$1 AND deleted_at IS NULL`, id)
	if err != nil {
		return fmt.Errorf("DeviceRepo.Delete: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("DeviceRepo.Delete: %w", err)
	}
	if n == 0 {
		return fmt.Errorf("DeviceRepo.Delete: %w", device.ErrDeviceNotFound)
	}

	_, err = tx.ExecContext(ctx,
		`UPDATE channels SET deleted_at=NOW(), updated_at=NOW() WHERE device_id=$1 AND deleted_at IS NULL`, id)
	if err != nil {
		return fmt.Errorf("DeviceRepo.Delete: cascade channels: %w", err)
	}

	return tx.Commit()
}
