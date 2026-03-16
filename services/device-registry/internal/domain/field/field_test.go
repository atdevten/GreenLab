package field

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewField(t *testing.T) {
	chID := uuid.New()

	t.Run("success", func(t *testing.T) {
		f, err := NewField(chID, "temperature", "Temperature", "°C", FieldTypeFloat, 1)
		require.NoError(t, err)
		assert.Equal(t, chID, f.ChannelID)
		assert.Equal(t, "temperature", f.Name)
		assert.Equal(t, "Temperature", f.Label)
		assert.Equal(t, "°C", f.Unit)
		assert.Equal(t, FieldTypeFloat, f.FieldType)
		assert.Equal(t, 1, f.Position)
		assert.NotEqual(t, uuid.Nil, f.ID)
	})

	t.Run("empty name returns error", func(t *testing.T) {
		f, err := NewField(chID, "", "Label", "unit", FieldTypeFloat, 1)
		assert.ErrorIs(t, err, ErrInvalidName)
		assert.Nil(t, f)
	})

	t.Run("position zero returns error", func(t *testing.T) {
		f, err := NewField(chID, "temp", "", "", FieldTypeFloat, 0)
		assert.ErrorIs(t, err, ErrInvalidPosition)
		assert.Nil(t, f)
	})

	t.Run("position greater than 8 returns error", func(t *testing.T) {
		f, err := NewField(chID, "temp", "", "", FieldTypeFloat, 9)
		assert.ErrorIs(t, err, ErrInvalidPosition)
		assert.Nil(t, f)
	})

	t.Run("invalid field type returns error", func(t *testing.T) {
		f, err := NewField(chID, "temp", "", "", "complex", 1)
		assert.ErrorIs(t, err, ErrInvalidFieldType)
		assert.Nil(t, f)
	})

	t.Run("empty field type defaults to float", func(t *testing.T) {
		f, err := NewField(chID, "temp", "", "", "", 1)
		require.NoError(t, err)
		assert.Equal(t, FieldTypeFloat, f.FieldType)
	})
}
