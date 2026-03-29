package http

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/greenlab/ingestion/internal/domain"
)

func TestResolveTDOffsets(t *testing.T) {
	baseTS := time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)
	fields := []string{"temperature", "humidity"}

	t.Run("nil td — all fields use baseTS", func(t *testing.T) {
		result, err := resolveTDOffsets(baseTS, nil, fields)
		require.NoError(t, err)
		require.Len(t, result, 2)
		assert.Equal(t, baseTS, *result["temperature"])
		assert.Equal(t, baseTS, *result["humidity"])
	})

	t.Run("empty td — all fields use baseTS", func(t *testing.T) {
		result, err := resolveTDOffsets(baseTS, []uint16{}, fields)
		require.NoError(t, err)
		require.Len(t, result, 2)
		assert.Equal(t, baseTS, *result["temperature"])
		assert.Equal(t, baseTS, *result["humidity"])
	})

	t.Run("valid td [0, 180] — field[0]=baseTS, field[1]=baseTS+180ms", func(t *testing.T) {
		result, err := resolveTDOffsets(baseTS, []uint16{0, 180}, fields)
		require.NoError(t, err)
		assert.Equal(t, baseTS, *result["temperature"])
		assert.Equal(t, baseTS.Add(180*time.Millisecond), *result["humidity"])
	})

	t.Run("td [0, 0] — both fields use baseTS", func(t *testing.T) {
		result, err := resolveTDOffsets(baseTS, []uint16{0, 0}, fields)
		require.NoError(t, err)
		assert.Equal(t, baseTS, *result["temperature"])
		assert.Equal(t, baseTS, *result["humidity"])
	})

	t.Run("max uint16 td — 65535ms offset applied", func(t *testing.T) {
		singleField := []string{"temp"}
		result, err := resolveTDOffsets(baseTS, []uint16{65535}, singleField)
		require.NoError(t, err)
		assert.Equal(t, baseTS.Add(65535*time.Millisecond), *result["temp"])
	})

	t.Run("len(td) != len(fields) — ErrTSDeltaInvalid", func(t *testing.T) {
		_, err := resolveTDOffsets(baseTS, []uint16{0, 100, 200}, fields)
		assert.ErrorIs(t, err, domain.ErrTSDeltaInvalid)
	})

	t.Run("len(td) == 1, len(fields) == 2 — ErrTSDeltaInvalid", func(t *testing.T) {
		_, err := resolveTDOffsets(baseTS, []uint16{100}, fields)
		assert.ErrorIs(t, err, domain.ErrTSDeltaInvalid)
	})

	t.Run("empty fields with empty td — returns empty map", func(t *testing.T) {
		result, err := resolveTDOffsets(baseTS, nil, []string{})
		require.NoError(t, err)
		assert.Empty(t, result)
	})
}
