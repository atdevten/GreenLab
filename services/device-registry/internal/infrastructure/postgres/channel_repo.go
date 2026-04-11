package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/greenlab/device-registry/internal/domain/channel"
)

type channelRow struct {
	ID            uuid.UUID                 `db:"id"`
	ShortID       int                       `db:"short_id"`
	WorkspaceID   uuid.UUID                 `db:"workspace_id"`
	DeviceID      *uuid.UUID                `db:"device_id"`
	Name          string                    `db:"name"`
	Description   string                    `db:"description"`
	Visibility    channel.ChannelVisibility `db:"visibility"`
	Tags          []byte                    `db:"tags"`
	RetentionDays int                       `db:"retention_days"`
	CreatedAt     time.Time                 `db:"created_at"`
	UpdatedAt     time.Time                 `db:"updated_at"`
	DeletedAt     *time.Time                `db:"deleted_at"`
}

func (r *channelRow) toDomain() *channel.Channel {
	return &channel.Channel{
		ID:            r.ID,
		ShortID:       r.ShortID,
		WorkspaceID:   r.WorkspaceID,
		DeviceID:      r.DeviceID,
		Name:          r.Name,
		Description:   r.Description,
		Visibility:    r.Visibility,
		Tags:          r.Tags,
		RetentionDays: r.RetentionDays,
		CreatedAt:     r.CreatedAt,
		UpdatedAt:     r.UpdatedAt,
		DeletedAt:     r.DeletedAt,
	}
}

func toChannelRow(ch *channel.Channel) *channelRow {
	return &channelRow{
		ID:            ch.ID,
		ShortID:       ch.ShortID,
		WorkspaceID:   ch.WorkspaceID,
		DeviceID:      ch.DeviceID,
		Name:          ch.Name,
		Description:   ch.Description,
		Visibility:    ch.Visibility,
		Tags:          ch.Tags,
		RetentionDays: ch.RetentionDays,
		CreatedAt:     ch.CreatedAt,
		UpdatedAt:     ch.UpdatedAt,
		DeletedAt:     ch.DeletedAt,
	}
}

type ChannelRepo struct{ db *sqlx.DB }

func NewChannelRepo(db *sqlx.DB) *ChannelRepo { return &ChannelRepo{db: db} }

func (r *ChannelRepo) Create(ctx context.Context, ch *channel.Channel) error {
	row := toChannelRow(ch)
	err := r.db.QueryRowContext(ctx, `
		INSERT INTO channels (id, workspace_id, device_id, name, description, visibility, tags)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING short_id`,
		row.ID, row.WorkspaceID, row.DeviceID, row.Name, row.Description, row.Visibility, row.Tags,
	).Scan(&ch.ShortID)
	if err != nil {
		return fmt.Errorf("ChannelRepo.Create: %w", err)
	}
	return nil
}

func (r *ChannelRepo) GetByID(ctx context.Context, id uuid.UUID) (*channel.Channel, error) {
	var row channelRow
	err := r.db.GetContext(ctx, &row,
		`SELECT id, short_id, workspace_id, device_id, name, description, visibility, tags, created_at, updated_at, deleted_at FROM channels WHERE id=$1 AND deleted_at IS NULL`, id)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("ChannelRepo.GetByID: %w", channel.ErrChannelNotFound)
	}
	if err != nil {
		return nil, fmt.Errorf("ChannelRepo.GetByID: %w", err)
	}
	return row.toDomain(), nil
}

type channelListRow struct {
	channelRow
	TotalCount int64 `db:"total_count"`
}

func (r *ChannelRepo) ListByWorkspace(ctx context.Context, workspaceID uuid.UUID, limit, offset int) ([]*channel.Channel, int64, error) {
	var rows []channelListRow
	err := r.db.SelectContext(ctx, &rows, `
		SELECT id, short_id, workspace_id, device_id, name, description, visibility, tags, created_at, updated_at, deleted_at,
		       COUNT(*) OVER() AS total_count
		FROM channels WHERE workspace_id=$1 AND deleted_at IS NULL ORDER BY created_at DESC LIMIT $2 OFFSET $3`,
		workspaceID, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("ChannelRepo.ListByWorkspace: %w", err)
	}
	var total int64
	channels := make([]*channel.Channel, len(rows))
	for i := range rows {
		if i == 0 {
			total = rows[i].TotalCount
		}
		channels[i] = rows[i].channelRow.toDomain()
	}
	return channels, total, nil
}

func (r *ChannelRepo) ListByDevice(ctx context.Context, deviceID uuid.UUID, limit, offset int) ([]*channel.Channel, int64, error) {
	var rows []channelListRow
	err := r.db.SelectContext(ctx, &rows, `
		SELECT id, short_id, workspace_id, device_id, name, description, visibility, tags, created_at, updated_at, deleted_at,
		       COUNT(*) OVER() AS total_count
		FROM channels WHERE device_id=$1 AND deleted_at IS NULL ORDER BY created_at DESC LIMIT $2 OFFSET $3`,
		deviceID, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("ChannelRepo.ListByDevice: %w", err)
	}
	var total int64
	channels := make([]*channel.Channel, len(rows))
	for i := range rows {
		if i == 0 {
			total = rows[i].TotalCount
		}
		channels[i] = rows[i].channelRow.toDomain()
	}
	return channels, total, nil
}

func (r *ChannelRepo) Update(ctx context.Context, ch *channel.Channel) error {
	_, err := r.db.ExecContext(ctx, `
		UPDATE channels SET name=$1, description=$2, visibility=$3, tags=$4, updated_at=NOW() WHERE id=$5`,
		ch.Name, ch.Description, ch.Visibility, ch.Tags, ch.ID)
	if err != nil {
		return fmt.Errorf("ChannelRepo.Update: %w", err)
	}
	return nil
}

func (r *ChannelRepo) Delete(ctx context.Context, id uuid.UUID) error {
	res, err := r.db.ExecContext(ctx,
		`UPDATE channels SET deleted_at=NOW(), updated_at=NOW() WHERE id=$1 AND deleted_at IS NULL`, id)
	if err != nil {
		return fmt.Errorf("ChannelRepo.Delete: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("ChannelRepo.Delete: %w", err)
	}
	if n == 0 {
		return fmt.Errorf("ChannelRepo.Delete: %w", channel.ErrChannelNotFound)
	}
	return nil
}
