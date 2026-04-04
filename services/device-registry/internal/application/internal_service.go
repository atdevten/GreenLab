package application

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/greenlab/device-registry/internal/domain/device"
)

// FieldEntry describes one field in the compact-format index map.
type FieldEntry struct {
	Index uint8
	Name  string
	Type  string
}

// ValidateAPIKeyResult is the auth + schema result returned by InternalService.
type ValidateAPIKeyResult struct {
	DeviceID      string
	Fields        []FieldEntry
	SchemaVersion uint32
}

// ResolveChannelResult is returned by ResolveChannelByAPIKey: the first channel
// belonging to the device identified by the API key, plus its schema.
type ResolveChannelResult struct {
	ChannelID string
	ValidateAPIKeyResult
}

// internalDeviceStore is the minimal query interface needed by InternalService.
type internalDeviceStore interface {
	ValidateAPIKey(ctx context.Context, apiKey, channelID string) (ValidateAPIKeyResult, error)
	ResolveChannelByAPIKey(ctx context.Context, apiKey string) (ResolveChannelResult, error)
}

// InternalService handles internal cross-service queries (no JWT, machine-to-machine).
type InternalService struct {
	store internalDeviceStore
}

func NewInternalService(store internalDeviceStore) *InternalService {
	return &InternalService{store: store}
}

// ResolveChannelByAPIKey resolves the first channel owned by the device identified
// by apiKey and returns its schema. Returns device.ErrDeviceNotFound when not found.
func (s *InternalService) ResolveChannelByAPIKey(ctx context.Context, apiKey string) (ResolveChannelResult, error) {
	result, err := s.store.ResolveChannelByAPIKey(ctx, apiKey)
	if err != nil {
		if errors.Is(err, device.ErrDeviceNotFound) {
			return ResolveChannelResult{}, fmt.Errorf("InternalService.ResolveChannelByAPIKey: %w", device.ErrDeviceNotFound)
		}
		return ResolveChannelResult{}, fmt.Errorf("InternalService.ResolveChannelByAPIKey: %w", err)
	}
	return result, nil
}

func (s *InternalService) ValidateAPIKey(ctx context.Context, apiKey, channelID string) (ValidateAPIKeyResult, error) {
	result, err := s.store.ValidateAPIKey(ctx, apiKey, channelID)
	if err != nil {
		if errors.Is(err, device.ErrDeviceNotFound) {
			return ValidateAPIKeyResult{}, fmt.Errorf("InternalService.ValidateAPIKey: %w", device.ErrDeviceNotFound)
		}
		if errors.Is(err, sql.ErrNoRows) {
			return ValidateAPIKeyResult{}, fmt.Errorf("InternalService.ValidateAPIKey: %w", device.ErrDeviceNotFound)
		}
		return ValidateAPIKeyResult{}, fmt.Errorf("InternalService.ValidateAPIKey: %w", err)
	}
	return result, nil
}
