package application

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/greenlab/device-registry/internal/domain/device"
)

// --- mock internalDeviceStore ---

type mockInternalDeviceStore struct{ mock.Mock }

func (m *mockInternalDeviceStore) ValidateAPIKey(ctx context.Context, apiKey, channelID string) (ValidateAPIKeyResult, error) {
	args := m.Called(ctx, apiKey, channelID)
	return args.Get(0).(ValidateAPIKeyResult), args.Error(1)
}

func (m *mockInternalDeviceStore) ResolveChannelByAPIKey(ctx context.Context, apiKey string) (ResolveChannelResult, error) {
	args := m.Called(ctx, apiKey)
	return args.Get(0).(ResolveChannelResult), args.Error(1)
}

// --- tests ---

func TestInternalService_ResolveChannelByAPIKey(t *testing.T) {
	ctx := context.Background()

	t.Run("success — returns result from store", func(t *testing.T) {
		store := &mockInternalDeviceStore{}
		svc := NewInternalService(store)

		expected := ResolveChannelResult{
			ChannelID: "chan-1",
			ValidateAPIKeyResult: ValidateAPIKeyResult{
				DeviceID: "dev-1",
				Fields: []FieldEntry{
					{Index: 1, Name: "temperature", Type: "float"},
				},
				SchemaVersion: 1,
			},
		}
		store.On("ResolveChannelByAPIKey", ctx, "key-abc").Return(expected, nil)

		result, err := svc.ResolveChannelByAPIKey(ctx, "key-abc")
		require.NoError(t, err)
		assert.Equal(t, expected, result)
		store.AssertExpectations(t)
	})

	t.Run("ErrDeviceNotFound from store is wrapped and returned", func(t *testing.T) {
		store := &mockInternalDeviceStore{}
		svc := NewInternalService(store)

		store.On("ResolveChannelByAPIKey", ctx, "bad-key").
			Return(ResolveChannelResult{}, device.ErrDeviceNotFound)

		_, err := svc.ResolveChannelByAPIKey(ctx, "bad-key")
		require.Error(t, err)
		assert.ErrorIs(t, err, device.ErrDeviceNotFound)
		store.AssertExpectations(t)
	})

	t.Run("generic store error is wrapped and returned", func(t *testing.T) {
		store := &mockInternalDeviceStore{}
		svc := NewInternalService(store)

		dbErr := errors.New("db timeout")
		store.On("ResolveChannelByAPIKey", ctx, "key").Return(ResolveChannelResult{}, dbErr)

		_, err := svc.ResolveChannelByAPIKey(ctx, "key")
		require.Error(t, err)
		assert.ErrorIs(t, err, dbErr)
		store.AssertExpectations(t)
	})
}

func TestInternalService_ValidateAPIKey(t *testing.T) {
	ctx := context.Background()

	t.Run("success — returns result from store", func(t *testing.T) {
		store := &mockInternalDeviceStore{}
		svc := NewInternalService(store)

		expected := ValidateAPIKeyResult{
			DeviceID: "dev-1",
			Fields: []FieldEntry{
				{Index: 1, Name: "temperature", Type: "float"},
			},
			SchemaVersion: 1,
		}
		store.On("ValidateAPIKey", ctx, "key-abc", "chan-123").Return(expected, nil)

		result, err := svc.ValidateAPIKey(ctx, "key-abc", "chan-123")
		require.NoError(t, err)
		assert.Equal(t, expected, result)
		store.AssertExpectations(t)
	})

	t.Run("ErrDeviceNotFound from store is wrapped and returned", func(t *testing.T) {
		store := &mockInternalDeviceStore{}
		svc := NewInternalService(store)

		store.On("ValidateAPIKey", ctx, "bad-key", "chan-123").
			Return(ValidateAPIKeyResult{}, device.ErrDeviceNotFound)

		_, err := svc.ValidateAPIKey(ctx, "bad-key", "chan-123")
		require.Error(t, err)
		assert.ErrorIs(t, err, device.ErrDeviceNotFound)
		store.AssertExpectations(t)
	})

	t.Run("generic store error is wrapped and returned", func(t *testing.T) {
		store := &mockInternalDeviceStore{}
		svc := NewInternalService(store)

		dbErr := errors.New("database connection error")
		store.On("ValidateAPIKey", ctx, "key", "chan").Return(ValidateAPIKeyResult{}, dbErr)

		_, err := svc.ValidateAPIKey(ctx, "key", "chan")
		require.Error(t, err)
		assert.ErrorIs(t, err, dbErr)
		store.AssertExpectations(t)
	})
}
