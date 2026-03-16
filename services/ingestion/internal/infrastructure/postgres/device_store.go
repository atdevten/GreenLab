package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/greenlab/ingestion/internal/domain"
)

const deviceStatusActive = "active"

// DeviceStore looks up devices from Postgres by API key.
type DeviceStore struct {
	db *sql.DB
}

func NewDeviceStore(db *sql.DB) *DeviceStore {
	return &DeviceStore{db: db}
}

// GetByAPIKey returns the device ID and channel ID for an active device with the given API key.
func (s *DeviceStore) GetByAPIKey(ctx context.Context, apiKey string) (deviceID, channelID string, err error) {
	err = s.db.QueryRowContext(ctx,
		`SELECT id, channel_id FROM devices WHERE api_key=$1 AND status=$2`, apiKey, deviceStatusActive,
	).Scan(&deviceID, &channelID)
	if errors.Is(err, sql.ErrNoRows) {
		return "", "", fmt.Errorf("DeviceStore.GetByAPIKey: %w", domain.ErrDeviceNotFound)
	}
	if err != nil {
		return "", "", fmt.Errorf("DeviceStore.GetByAPIKey: %w", err)
	}
	return deviceID, channelID, nil
}
