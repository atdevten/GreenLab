package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/jmoiron/sqlx"
	"github.com/greenlab/device-registry/internal/application"
	"github.com/greenlab/device-registry/internal/domain/device"
)

// InternalRepo services internal cross-service queries.
type InternalRepo struct{ db *sqlx.DB }

func NewInternalRepo(db *sqlx.DB) *InternalRepo { return &InternalRepo{db: db} }

type validateAPIKeyRow struct {
	DeviceID  string `db:"device_id"`
	FieldName string `db:"name"`
	FieldType string `db:"field_type"`
	Position  int    `db:"position"`
}

// ValidateAPIKey checks api_key + channel_id and returns the schema for that channel.
// Returns device.ErrDeviceNotFound when no active device matches.
func (r *InternalRepo) ValidateAPIKey(ctx context.Context, apiKey, channelID string) (application.ValidateAPIKeyResult, error) {
	var rows []validateAPIKeyRow
	err := r.db.SelectContext(ctx, &rows, `
		SELECT d.id as device_id, f.name, f.field_type, f.position
		FROM devices d
		JOIN channels c ON c.device_id = d.id
		JOIN fields f ON f.channel_id = c.id
		WHERE d.api_key = $1 AND d.status = 'active' AND d.deleted_at IS NULL AND c.id = $2
		ORDER BY f.position`,
		apiKey, channelID)
	if err != nil {
		return application.ValidateAPIKeyResult{}, fmt.Errorf("InternalRepo.ValidateAPIKey: %w", err)
	}

	// Check if device+channel combination exists (might have no fields yet)
	// We do a secondary query to confirm the device+channel pairing when fields are absent.
	if len(rows) == 0 {
		var deviceID string
		err = r.db.QueryRowContext(ctx, `
			SELECT d.id FROM devices d
			JOIN channels c ON c.device_id = d.id
			WHERE d.api_key = $1 AND d.status = 'active' AND d.deleted_at IS NULL AND c.id = $2`,
			apiKey, channelID).Scan(&deviceID)
		if errors.Is(err, sql.ErrNoRows) {
			return application.ValidateAPIKeyResult{}, fmt.Errorf("InternalRepo.ValidateAPIKey: %w", device.ErrDeviceNotFound)
		}
		if err != nil {
			return application.ValidateAPIKeyResult{}, fmt.Errorf("InternalRepo.ValidateAPIKey: %w", err)
		}
		// Valid device+channel with no fields yet
		return application.ValidateAPIKeyResult{
			DeviceID:      deviceID,
			Fields:        []application.FieldEntry{},
			SchemaVersion: 1,
		}, nil
	}

	fields := make([]application.FieldEntry, len(rows))
	for i, row := range rows {
		fields[i] = application.FieldEntry{
			Index: uint8(row.Position), //nolint:gosec // Position is constrained 1-8 by domain
			Name:  row.FieldName,
			Type:  row.FieldType,
		}
	}

	return application.ValidateAPIKeyResult{
		DeviceID:      rows[0].DeviceID,
		Fields:        fields,
		SchemaVersion: 1,
	}, nil
}
