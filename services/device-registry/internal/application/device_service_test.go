package application

import (
	"context"
	"errors"
	"log/slog"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/greenlab/device-registry/internal/domain/device"
	mockdevice "github.com/greenlab/device-registry/internal/mocks/device"
)

func newTestDeviceService(t *testing.T) (*DeviceService, *mockdevice.MockDeviceRepository, *mockdevice.MockDeviceCacheRepository) {
	t.Helper()
	repo := mockdevice.NewMockDeviceRepository(t)
	cache := mockdevice.NewMockDeviceCacheRepository(t)
	svc := NewDeviceService(repo, cache, slog.Default())
	return svc, repo, cache
}

func TestCreateDevice(t *testing.T) {
	ctx := context.Background()

	t.Run("success", func(t *testing.T) {
		svc, repo, cache := newTestDeviceService(t)
		wsID := uuid.New()

		repo.On("Create", ctx, mock.AnythingOfType("*device.Device")).Return(nil)
		cache.On("SetDevice", ctx, mock.AnythingOfType("*device.Device")).Return(nil)

		d, err := svc.CreateDevice(ctx, CreateDeviceInput{
			WorkspaceID: wsID.String(),
			Name:        "My Device",
			Description: "desc",
		})
		require.NoError(t, err)
		assert.Equal(t, "My Device", d.Name)
		repo.AssertExpectations(t)
		cache.AssertExpectations(t)
	})

	t.Run("invalid workspace_id", func(t *testing.T) {
		svc, _, _ := newTestDeviceService(t)
		d, err := svc.CreateDevice(ctx, CreateDeviceInput{WorkspaceID: "bad-uuid", Name: "name"})
		assert.Error(t, err)
		assert.Nil(t, d)
	})

	t.Run("empty name returns domain error", func(t *testing.T) {
		svc, _, _ := newTestDeviceService(t)
		d, err := svc.CreateDevice(ctx, CreateDeviceInput{WorkspaceID: uuid.New().String(), Name: ""})
		assert.ErrorIs(t, err, device.ErrInvalidName)
		assert.Nil(t, d)
	})

	t.Run("whitespace name returns domain error", func(t *testing.T) {
		svc, _, _ := newTestDeviceService(t)
		d, err := svc.CreateDevice(ctx, CreateDeviceInput{WorkspaceID: uuid.New().String(), Name: "   "})
		assert.ErrorIs(t, err, device.ErrInvalidName)
		assert.Nil(t, d)
	})

	t.Run("repo error is propagated", func(t *testing.T) {
		svc, repo, _ := newTestDeviceService(t)
		dbErr := errors.New("db error")
		repo.On("Create", ctx, mock.AnythingOfType("*device.Device")).Return(dbErr)

		d, err := svc.CreateDevice(ctx, CreateDeviceInput{WorkspaceID: uuid.New().String(), Name: "My Device"})
		assert.Error(t, err)
		assert.Nil(t, d)
		repo.AssertExpectations(t)
	})
}

func TestGetDevice(t *testing.T) {
	ctx := context.Background()

	t.Run("success", func(t *testing.T) {
		svc, repo, _ := newTestDeviceService(t)
		id := uuid.New()
		expected := &device.Device{ID: id, Name: "My Device"}
		repo.On("GetByID", ctx, id).Return(expected, nil)

		d, err := svc.GetDevice(ctx, id.String())
		require.NoError(t, err)
		assert.Equal(t, expected, d)
		repo.AssertExpectations(t)
	})

	t.Run("invalid id", func(t *testing.T) {
		svc, _, _ := newTestDeviceService(t)
		d, err := svc.GetDevice(ctx, "not-a-uuid")
		assert.Error(t, err)
		assert.Nil(t, d)
	})

	t.Run("not found", func(t *testing.T) {
		svc, repo, _ := newTestDeviceService(t)
		id := uuid.New()
		repo.On("GetByID", ctx, id).Return(nil, device.ErrDeviceNotFound)

		d, err := svc.GetDevice(ctx, id.String())
		assert.ErrorIs(t, err, device.ErrDeviceNotFound)
		assert.Nil(t, d)
		repo.AssertExpectations(t)
	})
}

func TestUpdateDevice(t *testing.T) {
	ctx := context.Background()

	t.Run("success", func(t *testing.T) {
		svc, repo, cache := newTestDeviceService(t)
		id := uuid.New()
		existing := &device.Device{ID: id, Name: "Old Name", Status: device.DeviceStatusActive}
		repo.On("GetByID", ctx, id).Return(existing, nil)
		repo.On("Update", ctx, mock.AnythingOfType("*device.Device")).Return(nil)
		cache.On("SetDevice", ctx, mock.AnythingOfType("*device.Device")).Return(nil)

		d, err := svc.UpdateDevice(ctx, id.String(), UpdateDeviceInput{Name: "New Name"})
		require.NoError(t, err)
		assert.Equal(t, "New Name", d.Name)
		repo.AssertExpectations(t)
		cache.AssertExpectations(t)
	})

	t.Run("whitespace name returns domain error", func(t *testing.T) {
		svc, repo, _ := newTestDeviceService(t)
		id := uuid.New()
		existing := &device.Device{ID: id, Name: "Name", Status: device.DeviceStatusActive}
		repo.On("GetByID", ctx, id).Return(existing, nil)

		d, err := svc.UpdateDevice(ctx, id.String(), UpdateDeviceInput{Name: "   "})
		assert.ErrorIs(t, err, device.ErrInvalidName)
		assert.Nil(t, d)
		repo.AssertExpectations(t)
	})

	t.Run("invalid status returns error", func(t *testing.T) {
		svc, repo, _ := newTestDeviceService(t)
		id := uuid.New()
		existing := &device.Device{ID: id, Name: "Name", Status: device.DeviceStatusActive}
		repo.On("GetByID", ctx, id).Return(existing, nil)

		d, err := svc.UpdateDevice(ctx, id.String(), UpdateDeviceInput{Status: "badstatus"})
		assert.ErrorIs(t, err, device.ErrInvalidStatus)
		assert.Nil(t, d)
		repo.AssertExpectations(t)
	})
}

func TestRotateAPIKey(t *testing.T) {
	ctx := context.Background()

	t.Run("success", func(t *testing.T) {
		svc, repo, cache := newTestDeviceService(t)
		id := uuid.New()
		existing := &device.Device{ID: id, Name: "My Device", APIKey: "ts_old"}
		repo.On("GetByID", ctx, id).Return(existing, nil)
		repo.On("Update", ctx, mock.AnythingOfType("*device.Device")).Return(nil)
		cache.On("SetDevice", ctx, mock.AnythingOfType("*device.Device")).Return(nil)

		d, err := svc.RotateAPIKey(ctx, id.String())
		require.NoError(t, err)
		assert.NotEqual(t, "ts_old", d.APIKey)
		repo.AssertExpectations(t)
		cache.AssertExpectations(t)
	})
}

func TestDeleteDevice(t *testing.T) {
	ctx := context.Background()

	t.Run("success — db deleted before cache eviction", func(t *testing.T) {
		svc, repo, cache := newTestDeviceService(t)
		id := uuid.New()
		apiKey := "ts_somekey"
		existing := &device.Device{ID: id, APIKey: apiKey}
		repo.On("GetByID", ctx, id).Return(existing, nil)
		deleteCall := repo.On("Delete", ctx, id).Return(nil)
		cache.On("DeleteDevice", ctx, id.String(), apiKey).Return(nil).NotBefore(deleteCall)

		err := svc.DeleteDevice(ctx, id.String())
		require.NoError(t, err)
		repo.AssertExpectations(t)
		cache.AssertExpectations(t)
	})

	t.Run("not found", func(t *testing.T) {
		svc, repo, _ := newTestDeviceService(t)
		id := uuid.New()
		repo.On("GetByID", ctx, id).Return(nil, device.ErrDeviceNotFound)

		err := svc.DeleteDevice(ctx, id.String())
		assert.ErrorIs(t, err, device.ErrDeviceNotFound)
		repo.AssertExpectations(t)
	})

	t.Run("repo delete error — cache not touched", func(t *testing.T) {
		svc, repo, _ := newTestDeviceService(t)
		id := uuid.New()
		existing := &device.Device{ID: id, APIKey: "ts_somekey"}
		repo.On("GetByID", ctx, id).Return(existing, nil)
		repo.On("Delete", ctx, id).Return(errors.New("db error"))

		err := svc.DeleteDevice(ctx, id.String())
		assert.Error(t, err)
		repo.AssertExpectations(t)
	})
}

func TestValidateAPIKey(t *testing.T) {
	ctx := context.Background()

	t.Run("cache hit", func(t *testing.T) {
		svc, _, cache := newTestDeviceService(t)
		apiKey := "ts_somekey"
		expected := &device.Device{APIKey: apiKey}
		cache.On("GetDeviceByAPIKey", ctx, apiKey).Return(expected, nil)

		d, err := svc.ValidateAPIKey(ctx, apiKey)
		require.NoError(t, err)
		assert.Equal(t, expected, d)
		cache.AssertExpectations(t)
	})

	t.Run("cache miss falls through to repo", func(t *testing.T) {
		svc, repo, cache := newTestDeviceService(t)
		apiKey := "ts_somekey"
		expected := &device.Device{APIKey: apiKey}
		cache.On("GetDeviceByAPIKey", ctx, apiKey).Return(nil, device.ErrCacheMiss)
		repo.On("GetByAPIKey", ctx, apiKey).Return(expected, nil)
		cache.On("SetDevice", ctx, expected).Return(nil)

		d, err := svc.ValidateAPIKey(ctx, apiKey)
		require.NoError(t, err)
		assert.Equal(t, expected, d)
		repo.AssertExpectations(t)
		cache.AssertExpectations(t)
	})

	t.Run("unexpected cache error falls through to repo", func(t *testing.T) {
		svc, repo, cache := newTestDeviceService(t)
		apiKey := "ts_somekey"
		expected := &device.Device{APIKey: apiKey}
		cache.On("GetDeviceByAPIKey", ctx, apiKey).Return(nil, errors.New("redis: connection refused"))
		repo.On("GetByAPIKey", ctx, apiKey).Return(expected, nil)
		cache.On("SetDevice", ctx, expected).Return(nil)

		d, err := svc.ValidateAPIKey(ctx, apiKey)
		require.NoError(t, err)
		assert.Equal(t, expected, d)
		repo.AssertExpectations(t)
		cache.AssertExpectations(t)
	})

	t.Run("not found", func(t *testing.T) {
		svc, repo, cache := newTestDeviceService(t)
		apiKey := "ts_unknown"
		cache.On("GetDeviceByAPIKey", ctx, apiKey).Return(nil, device.ErrCacheMiss)
		repo.On("GetByAPIKey", ctx, apiKey).Return(nil, device.ErrDeviceNotFound)

		d, err := svc.ValidateAPIKey(ctx, apiKey)
		assert.ErrorIs(t, err, device.ErrDeviceNotFound)
		assert.Nil(t, d)
		repo.AssertExpectations(t)
		cache.AssertExpectations(t)
	})
}
