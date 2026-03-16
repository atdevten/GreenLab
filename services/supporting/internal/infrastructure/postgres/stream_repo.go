package postgres

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/greenlab/supporting/internal/domain/video"
)

// streamRow is the DB-layer representation of a video stream.
type streamRow struct {
	ID           uuid.UUID          `db:"id"`
	DeviceID     uuid.UUID          `db:"device_id"`
	WorkspaceID  uuid.UUID          `db:"workspace_id"`
	Name         string             `db:"name"`
	Description  string             `db:"description"`
	Protocol     video.StreamProtocol `db:"protocol"`
	SourceURL    string             `db:"source_url"`
	StorageKey   string             `db:"storage_key"`
	Status       video.StreamStatus `db:"status"`
	ThumbnailURL string             `db:"thumbnail_url"`
	DurationSec  int                `db:"duration_sec"`
	CreatedAt    time.Time          `db:"created_at"`
	UpdatedAt    time.Time          `db:"updated_at"`
}

func toStream(r *streamRow) *video.Stream {
	return &video.Stream{
		ID:           r.ID,
		DeviceID:     r.DeviceID,
		WorkspaceID:  r.WorkspaceID,
		Name:         r.Name,
		Description:  r.Description,
		Protocol:     r.Protocol,
		SourceURL:    r.SourceURL,
		StorageKey:   r.StorageKey,
		Status:       r.Status,
		ThumbnailURL: r.ThumbnailURL,
		DurationSec:  r.DurationSec,
		CreatedAt:    r.CreatedAt,
		UpdatedAt:    r.UpdatedAt,
	}
}

func fromStream(s *video.Stream) *streamRow {
	return &streamRow{
		ID:           s.ID,
		DeviceID:     s.DeviceID,
		WorkspaceID:  s.WorkspaceID,
		Name:         s.Name,
		Description:  s.Description,
		Protocol:     s.Protocol,
		SourceURL:    s.SourceURL,
		StorageKey:   s.StorageKey,
		Status:       s.Status,
		ThumbnailURL: s.ThumbnailURL,
		DurationSec:  s.DurationSec,
		CreatedAt:    s.CreatedAt,
		UpdatedAt:    s.UpdatedAt,
	}
}

type StreamRepo struct{ db *sqlx.DB }

func NewStreamRepo(db *sqlx.DB) *StreamRepo { return &StreamRepo{db: db} }

func (r *StreamRepo) Create(ctx context.Context, s *video.Stream) error {
	_, err := r.db.NamedExecContext(ctx, `
		INSERT INTO video_streams (id, device_id, workspace_id, name, description, protocol, source_url, storage_key, status, thumbnail_url, duration_sec, created_at, updated_at)
		VALUES (:id, :device_id, :workspace_id, :name, :description, :protocol, :source_url, :storage_key, :status, :thumbnail_url, :duration_sec, :created_at, :updated_at)`,
		fromStream(s))
	return err
}

func (r *StreamRepo) GetByID(ctx context.Context, id uuid.UUID) (*video.Stream, error) {
	var row streamRow
	err := r.db.GetContext(ctx, &row,
		`SELECT id, device_id, workspace_id, name, description, protocol, source_url, storage_key, status, thumbnail_url, duration_sec, created_at, updated_at FROM video_streams WHERE id=$1`,
		id)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, video.ErrStreamNotFound
	}
	if err != nil {
		return nil, err
	}
	return toStream(&row), nil
}

func (r *StreamRepo) ListByDevice(ctx context.Context, deviceID uuid.UUID, limit, offset int) ([]*video.Stream, int64, error) {
	var rows []*streamRow
	err := r.db.SelectContext(ctx, &rows,
		`SELECT id, device_id, workspace_id, name, description, protocol, source_url, storage_key, status, thumbnail_url, duration_sec, created_at, updated_at FROM video_streams WHERE device_id=$1 ORDER BY created_at DESC LIMIT $2 OFFSET $3`,
		deviceID, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	var total int64
	if err := r.db.GetContext(ctx, &total, `SELECT COUNT(*) FROM video_streams WHERE device_id=$1`, deviceID); err != nil {
		return nil, 0, err
	}
	streams := make([]*video.Stream, len(rows))
	for i, row := range rows {
		streams[i] = toStream(row)
	}
	return streams, total, nil
}

func (r *StreamRepo) ListByWorkspace(ctx context.Context, workspaceID uuid.UUID, limit, offset int) ([]*video.Stream, int64, error) {
	var rows []*streamRow
	err := r.db.SelectContext(ctx, &rows,
		`SELECT id, device_id, workspace_id, name, description, protocol, source_url, storage_key, status, thumbnail_url, duration_sec, created_at, updated_at FROM video_streams WHERE workspace_id=$1 ORDER BY created_at DESC LIMIT $2 OFFSET $3`,
		workspaceID, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	var total int64
	if err := r.db.GetContext(ctx, &total, `SELECT COUNT(*) FROM video_streams WHERE workspace_id=$1`, workspaceID); err != nil {
		return nil, 0, err
	}
	streams := make([]*video.Stream, len(rows))
	for i, row := range rows {
		streams[i] = toStream(row)
	}
	return streams, total, nil
}

func (r *StreamRepo) Update(ctx context.Context, s *video.Stream) error {
	_, err := r.db.ExecContext(ctx, `
		UPDATE video_streams SET name=$1, description=$2, status=$3, storage_key=$4, thumbnail_url=$5, duration_sec=$6, updated_at=NOW() WHERE id=$7`,
		s.Name, s.Description, s.Status, s.StorageKey, s.ThumbnailURL, s.DurationSec, s.ID)
	return err
}

func (r *StreamRepo) Delete(ctx context.Context, id uuid.UUID) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM video_streams WHERE id=$1`, id)
	return err
}
