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
}
