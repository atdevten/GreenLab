package device

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewDevice(t *testing.T) {
	wsID := uuid.New()

	t.Run("success", func(t *testing.T) {
		d, err := NewDevice(wsID, "My Device", "A test device")
		require.NoError(t, err)
		assert.Equal(t, wsID, d.WorkspaceID)
		assert.Equal(t, "My Device", d.Name)
		assert.Equal(t, "A test device", d.Description)
		assert.Equal(t, DeviceStatusActive, d.Status)
		assert.NotEmpty(t, d.APIKey)
		assert.NotEqual(t, uuid.Nil, d.ID)
	})

	t.Run("empty name returns error", func(t *testing.T) {
		d, err := NewDevice(wsID, "", "some description")
		assert.ErrorIs(t, err, ErrInvalidName)
		assert.Nil(t, d)
	})
}

func TestRotateAPIKey(t *testing.T) {
	wsID := uuid.New()
	d, err := NewDevice(wsID, "My Device", "")
	require.NoError(t, err)

	originalKey := d.APIKey
	err = d.RotateAPIKey()
	require.NoError(t, err)
	assert.NotEqual(t, originalKey, d.APIKey)
	assert.NotEmpty(t, d.APIKey)
}

func TestIsActive(t *testing.T) {
	wsID := uuid.New()

	t.Run("active device returns true", func(t *testing.T) {
		d, err := NewDevice(wsID, "My Device", "")
		require.NoError(t, err)
		assert.True(t, d.IsActive())
	})

	t.Run("inactive device returns false", func(t *testing.T) {
		d, err := NewDevice(wsID, "My Device", "")
		require.NoError(t, err)
		d.Status = DeviceStatusInactive
		assert.False(t, d.IsActive())
	})

	t.Run("blocked device returns false", func(t *testing.T) {
		d, err := NewDevice(wsID, "My Device", "")
		require.NoError(t, err)
		d.Status = DeviceStatusBlocked
		assert.False(t, d.IsActive())
	})
}

func TestSetStatus(t *testing.T) {
	wsID := uuid.New()

	t.Run("set to inactive succeeds", func(t *testing.T) {
		d, err := NewDevice(wsID, "My Device", "")
		require.NoError(t, err)
		err = d.SetStatus(DeviceStatusInactive)
		require.NoError(t, err)
		assert.Equal(t, DeviceStatusInactive, d.Status)
	})

	t.Run("set to blocked succeeds", func(t *testing.T) {
		d, err := NewDevice(wsID, "My Device", "")
		require.NoError(t, err)
		err = d.SetStatus(DeviceStatusBlocked)
		require.NoError(t, err)
		assert.Equal(t, DeviceStatusBlocked, d.Status)
	})

	t.Run("set to active succeeds", func(t *testing.T) {
		d, err := NewDevice(wsID, "My Device", "")
		require.NoError(t, err)
		d.Status = DeviceStatusInactive
		err = d.SetStatus(DeviceStatusActive)
		require.NoError(t, err)
		assert.Equal(t, DeviceStatusActive, d.Status)
	})

	t.Run("invalid status returns error", func(t *testing.T) {
		d, err := NewDevice(wsID, "My Device", "")
		require.NoError(t, err)
		err = d.SetStatus("unknown")
		assert.ErrorIs(t, err, ErrInvalidStatus)
	})
}
