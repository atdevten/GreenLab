package http

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vmihailenco/msgpack/v5"
	"github.com/greenlab/ingestion/internal/domain"
)

func TestMsgPackDeserializer_Parse(t *testing.T) {
	d := newMsgPackDeserializer()

	assert.Equal(t, ctMsgPack, d.ContentType())

	t.Run("valid msgpack with fields, timestamp and td", func(t *testing.T) {
		ts := int64(1741426620000)
		payload := map[string]any{
			"f":  []float64{42.5, 28.1},
			"t":  ts,
			"td": []uint16{0, 180},
			"sv": uint32(1),
		}
		body, err := msgpack.Marshal(payload)
		require.NoError(t, err)

		batch, err := d.Parse(body)
		require.NoError(t, err)

		assert.Equal(t, uint32(1), batch.SchemaVersion)
		assert.Equal(t, float64(42.5), batch.FieldValues[1])
		assert.Equal(t, float64(28.1), batch.FieldValues[2])
		require.NotNil(t, batch.Timestamp)
		assert.Equal(t, time.UnixMilli(ts).UTC(), *batch.Timestamp)
	})

	t.Run("valid minimal msgpack — no timestamp", func(t *testing.T) {
		payload := map[string]any{
			"f":  []float64{10.0},
			"sv": uint32(2),
		}
		body, err := msgpack.Marshal(payload)
		require.NoError(t, err)

		batch, err := d.Parse(body)
		require.NoError(t, err)
		assert.Equal(t, float64(10.0), batch.FieldValues[1])
		assert.Nil(t, batch.Timestamp)
	})

	t.Run("empty f array returns ErrEmptyFields", func(t *testing.T) {
		payload := map[string]any{
			"f":  []float64{},
			"sv": uint32(1),
		}
		body, err := msgpack.Marshal(payload)
		require.NoError(t, err)

		_, err = d.Parse(body)
		assert.ErrorIs(t, err, domain.ErrEmptyFields)
	})

	t.Run("missing f key returns ErrEmptyFields", func(t *testing.T) {
		payload := map[string]any{
			"sv": uint32(1),
		}
		body, err := msgpack.Marshal(payload)
		require.NoError(t, err)

		_, err = d.Parse(body)
		assert.ErrorIs(t, err, domain.ErrEmptyFields)
	})

	t.Run("malformed bytes return parse error", func(t *testing.T) {
		_, err := d.Parse([]byte{0xc1, 0x00, 0xff}) // invalid msgpack
		require.Error(t, err)
	})
}
