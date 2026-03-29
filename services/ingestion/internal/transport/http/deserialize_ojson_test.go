package http

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/greenlab/ingestion/internal/domain"
)

func TestOJSONDeserializer_Parse(t *testing.T) {
	d := newOJSONDeserializer()

	assert.Equal(t, ctOJSON, d.ContentType())

	t.Run("valid compact JSON with two fields, timestamp and td offsets", func(t *testing.T) {
		body := []byte(`{"f":[42.5,28.1],"t":1741426620000,"td":[0,180],"sv":1}`)
		batch, err := d.Parse(body)
		require.NoError(t, err)

		assert.Equal(t, uint32(1), batch.SchemaVersion)
		assert.Equal(t, float64(42.5), batch.FieldValues[1])
		assert.Equal(t, float64(28.1), batch.FieldValues[2])

		require.NotNil(t, batch.Timestamp)
		assert.Equal(t, time.UnixMilli(1741426620000).UTC(), *batch.Timestamp)
		assert.Equal(t, []uint16{0, 180}, batch.TDOffsets)
	})

	t.Run("valid minimal — no timestamp, no td, single field", func(t *testing.T) {
		body := []byte(`{"f":[99.0],"sv":2}`)
		batch, err := d.Parse(body)
		require.NoError(t, err)

		assert.Equal(t, uint32(2), batch.SchemaVersion)
		assert.Equal(t, float64(99.0), batch.FieldValues[1])
		assert.Nil(t, batch.Timestamp)
		assert.Empty(t, batch.TDOffsets)
	})

	t.Run("missing f field — error", func(t *testing.T) {
		body := []byte(`{"t":1741426620000,"sv":1}`)
		_, err := d.Parse(body)
		assert.ErrorIs(t, err, domain.ErrEmptyFields)
	})

	t.Run("empty f array — error", func(t *testing.T) {
		body := []byte(`{"f":[],"sv":1}`)
		_, err := d.Parse(body)
		assert.ErrorIs(t, err, domain.ErrEmptyFields)
	})

	t.Run("malformed JSON — error", func(t *testing.T) {
		body := []byte(`{"f":[1.0,`)
		_, err := d.Parse(body)
		require.Error(t, err)
	})

	t.Run("invalid JSON type for f — error", func(t *testing.T) {
		body := []byte(`{"f":"not-an-array","sv":1}`)
		_, err := d.Parse(body)
		require.Error(t, err)
	})

	t.Run("schema version 0 — allowed (pre-TODO-028)", func(t *testing.T) {
		body := []byte(`{"f":[1.0],"sv":0}`)
		batch, err := d.Parse(body)
		require.NoError(t, err)
		assert.Equal(t, uint32(0), batch.SchemaVersion)
	})

	t.Run("three fields mapped to indices 1,2,3", func(t *testing.T) {
		body := []byte(`{"f":[1.1,2.2,3.3],"sv":1}`)
		batch, err := d.Parse(body)
		require.NoError(t, err)
		assert.Equal(t, float64(1.1), batch.FieldValues[1])
		assert.Equal(t, float64(2.2), batch.FieldValues[2])
		assert.Equal(t, float64(3.3), batch.FieldValues[3])
	})
}
