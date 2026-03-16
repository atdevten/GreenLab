package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/greenlab/device-registry/internal/domain/field"
)

type fieldRow struct {
	ID          uuid.UUID      `db:"id"`
	ChannelID   uuid.UUID      `db:"channel_id"`
	Name        string         `db:"name"`
	Label       string         `db:"label"`
	Unit        string         `db:"unit"`
	FieldType   field.FieldType `db:"field_type"`
	Position    int            `db:"position"`
	Description string         `db:"description"`
	CreatedAt   time.Time      `db:"created_at"`
	UpdatedAt   time.Time      `db:"updated_at"`
}

func (r *fieldRow) toDomain() *field.Field {
	return &field.Field{
		ID:          r.ID,
		ChannelID:   r.ChannelID,
		Name:        r.Name,
		Label:       r.Label,
		Unit:        r.Unit,
		FieldType:   r.FieldType,
		Position:    r.Position,
		Description: r.Description,
		CreatedAt:   r.CreatedAt,
		UpdatedAt:   r.UpdatedAt,
	}
}

func toFieldRow(f *field.Field) *fieldRow {
	return &fieldRow{
		ID:          f.ID,
		ChannelID:   f.ChannelID,
		Name:        f.Name,
		Label:       f.Label,
		Unit:        f.Unit,
		FieldType:   f.FieldType,
		Position:    f.Position,
		Description: f.Description,
		CreatedAt:   f.CreatedAt,
		UpdatedAt:   f.UpdatedAt,
	}
}

type FieldRepo struct{ db *sqlx.DB }

func NewFieldRepo(db *sqlx.DB) *FieldRepo { return &FieldRepo{db: db} }

func (r *FieldRepo) Create(ctx context.Context, f *field.Field) error {
	_, err := r.db.NamedExecContext(ctx, `
		INSERT INTO fields (id, channel_id, name, label, unit, field_type, position, description)
		VALUES (:id, :channel_id, :name, :label, :unit, :field_type, :position, :description)`, toFieldRow(f))
	if err != nil {
		return fmt.Errorf("FieldRepo.Create: %w", err)
	}
	return nil
}

func (r *FieldRepo) GetByID(ctx context.Context, id uuid.UUID) (*field.Field, error) {
	var row fieldRow
	err := r.db.GetContext(ctx, &row,
		`SELECT id, channel_id, name, label, unit, field_type, position, description, created_at, updated_at FROM fields WHERE id=$1`, id)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("FieldRepo.GetByID: %w", field.ErrFieldNotFound)
	}
	if err != nil {
		return nil, fmt.Errorf("FieldRepo.GetByID: %w", err)
	}
	return row.toDomain(), nil
}

func (r *FieldRepo) ListByChannel(ctx context.Context, channelID uuid.UUID) ([]*field.Field, error) {
	var rows []fieldRow
	err := r.db.SelectContext(ctx, &rows,
		`SELECT id, channel_id, name, label, unit, field_type, position, description, created_at, updated_at FROM fields WHERE channel_id=$1 ORDER BY position`, channelID)
	if err != nil {
		return nil, fmt.Errorf("FieldRepo.ListByChannel: %w", err)
	}
	fields := make([]*field.Field, len(rows))
	for i := range rows {
		fields[i] = rows[i].toDomain()
	}
	return fields, nil
}

func (r *FieldRepo) Update(ctx context.Context, f *field.Field) error {
	_, err := r.db.ExecContext(ctx, `
		UPDATE fields SET name=$1, label=$2, unit=$3, description=$4, updated_at=NOW() WHERE id=$5`,
		f.Name, f.Label, f.Unit, f.Description, f.ID)
	if err != nil {
		return fmt.Errorf("FieldRepo.Update: %w", err)
	}
	return nil
}

func (r *FieldRepo) Delete(ctx context.Context, id uuid.UUID) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM fields WHERE id=$1`, id)
	if err != nil {
		return fmt.Errorf("FieldRepo.Delete: %w", err)
	}
	return nil
}
