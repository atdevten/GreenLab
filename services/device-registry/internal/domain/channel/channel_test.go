package channel

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSoftDelete(t *testing.T) {
	wsID := uuid.New()

	t.Run("sets deleted_at and updates updated_at", func(t *testing.T) {
		ch, err := NewChannel(wsID, "Chan", "", ChannelVisibilityPrivate)
		require.NoError(t, err)
		assert.Nil(t, ch.DeletedAt)

		err = ch.SoftDelete()
		require.NoError(t, err)
		assert.NotNil(t, ch.DeletedAt)
		assert.Equal(t, ch.UpdatedAt, *ch.DeletedAt)
	})

	t.Run("double soft-delete returns not found", func(t *testing.T) {
		ch, err := NewChannel(wsID, "Chan", "", ChannelVisibilityPrivate)
		require.NoError(t, err)
		require.NoError(t, ch.SoftDelete())

		err = ch.SoftDelete()
		assert.ErrorIs(t, err, ErrChannelNotFound)
	})
}

func TestNewChannel(t *testing.T) {
	wsID := uuid.New()

	t.Run("success with explicit visibility", func(t *testing.T) {
		ch, err := NewChannel(wsID, "My Channel", "A description", ChannelVisibilityPublic)
		require.NoError(t, err)
		assert.Equal(t, wsID, ch.WorkspaceID)
		assert.Equal(t, "My Channel", ch.Name)
		assert.Equal(t, "A description", ch.Description)
		assert.Equal(t, ChannelVisibilityPublic, ch.Visibility)
		assert.NotEqual(t, uuid.Nil, ch.ID)
	})

	t.Run("empty name returns error", func(t *testing.T) {
		ch, err := NewChannel(wsID, "", "desc", ChannelVisibilityPrivate)
		assert.ErrorIs(t, err, ErrInvalidName)
		assert.Nil(t, ch)
	})

	t.Run("empty visibility defaults to private", func(t *testing.T) {
		ch, err := NewChannel(wsID, "My Channel", "desc", "")
		require.NoError(t, err)
		assert.Equal(t, ChannelVisibilityPrivate, ch.Visibility)
	})

	t.Run("invalid visibility returns error", func(t *testing.T) {
		ch, err := NewChannel(wsID, "My Channel", "desc", "protected")
		assert.ErrorIs(t, err, ErrInvalidVisibility)
		assert.Nil(t, ch)
	})

	t.Run("default retention days is 90", func(t *testing.T) {
		ch, err := NewChannel(wsID, "My Channel", "desc", ChannelVisibilityPublic)
		require.NoError(t, err)
		assert.Equal(t, DefaultRetentionDays, ch.RetentionDays)
	})
}

func TestSetRetentionDays(t *testing.T) {
	wsID := uuid.New()

	t.Run("valid retention sets value", func(t *testing.T) {
		ch, err := NewChannel(wsID, "Chan", "", ChannelVisibilityPrivate)
		require.NoError(t, err)

		err = ch.SetRetentionDays(30)
		require.NoError(t, err)
		assert.Equal(t, 30, ch.RetentionDays)
	})

	t.Run("minimum boundary (1) is valid", func(t *testing.T) {
		ch, err := NewChannel(wsID, "Chan", "", ChannelVisibilityPrivate)
		require.NoError(t, err)

		err = ch.SetRetentionDays(MinRetentionDays)
		require.NoError(t, err)
		assert.Equal(t, MinRetentionDays, ch.RetentionDays)
	})

	t.Run("maximum boundary (365) is valid", func(t *testing.T) {
		ch, err := NewChannel(wsID, "Chan", "", ChannelVisibilityPrivate)
		require.NoError(t, err)

		err = ch.SetRetentionDays(MaxRetentionDays)
		require.NoError(t, err)
		assert.Equal(t, MaxRetentionDays, ch.RetentionDays)
	})

	t.Run("zero returns invalid retention error", func(t *testing.T) {
		ch, err := NewChannel(wsID, "Chan", "", ChannelVisibilityPrivate)
		require.NoError(t, err)

		err = ch.SetRetentionDays(0)
		assert.ErrorIs(t, err, ErrInvalidRetention)
		assert.Equal(t, DefaultRetentionDays, ch.RetentionDays) // unchanged
	})

	t.Run("above maximum returns invalid retention error", func(t *testing.T) {
		ch, err := NewChannel(wsID, "Chan", "", ChannelVisibilityPrivate)
		require.NoError(t, err)

		err = ch.SetRetentionDays(366)
		assert.ErrorIs(t, err, ErrInvalidRetention)
	})

	t.Run("negative returns invalid retention error", func(t *testing.T) {
		ch, err := NewChannel(wsID, "Chan", "", ChannelVisibilityPrivate)
		require.NoError(t, err)

		err = ch.SetRetentionDays(-1)
		assert.ErrorIs(t, err, ErrInvalidRetention)
	})
}
