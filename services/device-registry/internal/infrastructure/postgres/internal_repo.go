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

type resolveChannelRow struct {
	DeviceID  string         `db:"device_id"`
	ChannelID string         `db:"channel_id"`
	FieldName sql.NullString `db:"name"`
	FieldType sql.NullString `db:"field_type"`
	Position  sql.NullInt32  `db:"position"`
}

// ResolveChannelByAPIKey looks up the first channel belonging to the device identified by
// apiKey and returns the combined result. Returns device.ErrDeviceNotFound when no active
// device matches.
func (r *InternalRepo) ResolveChannelByAPIKey(ctx context.Context, apiKey string) (application.ResolveChannelResult, error) {
	var rows []resolveChannelRow
	err := r.db.SelectContext(ctx, &rows, `
		SELECT d.id as device_id, c.id as channel_id, f.name, f.field_type, f.position
		FROM devices d
		JOIN channels c ON c.device_id = d.id
		LEFT JOIN fields f ON f.channel_id = c.id
		WHERE d.api_key = $1 AND d.status = 'active' AND d.deleted_at IS NULL
		ORDER BY c.created_at ASC, f.position ASC
		LIMIT 100`,
		apiKey)
	if err != nil {
		return application.ResolveChannelResult{}, fmt.Errorf("InternalRepo.ResolveChannelByAPIKey: %w", err)
	}
	if len(rows) == 0 {
		return application.ResolveChannelResult{}, fmt.Errorf("InternalRepo.ResolveChannelByAPIKey: %w", device.ErrDeviceNotFound)
	}

	deviceID := rows[0].DeviceID
	channelID := rows[0].ChannelID

	var fields []application.FieldEntry
	for _, row := range rows {
		if !row.FieldName.Valid {
			continue
		}
		fields = append(fields, application.FieldEntry{
			Index: uint8(row.Position.Int32), //nolint:gosec // Position is constrained 1-8 by domain
			Name:  row.FieldName.String,
			Type:  row.FieldType.String,
		})
	}
	if fields == nil {
		fields = []application.FieldEntry{}
	}

	return application.ResolveChannelResult{
		ChannelID: channelID,
		ValidateAPIKeyResult: application.ValidateAPIKeyResult{
			DeviceID:      deviceID,
			Fields:        fields,
			SchemaVersion: 1,
		},
	}, nil
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
