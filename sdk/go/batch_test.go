package sdk

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vmihailenco/msgpack/v5"
)

// makeSchema builds a channelSchema with the given field names at indices 1, 2, ...
func makeSchema(fieldNames ...string) channelSchema {
	fields := make([]fieldEntry, len(fieldNames))
	nm := make(map[string]uint8, len(fieldNames))
	im := make(map[uint8]string, len(fieldNames))
	for i, name := range fieldNames {
		idx := uint8(i + 1)
		fields[i] = fieldEntry{Index: idx, Name: name, Type: "float"}
		nm[name] = idx
		im[idx] = name
	}
	return channelSchema{
		nameToIndex:   nm,
		indexToName:   im,
		schemaVersion: 1,
		orderedFields: fields,
	}
}

func TestBuildBatches_SingleField(t *testing.T) {
	schema := makeSchema("temperature")
	ts := time.Unix(1700000000, 0).UTC()

	readings := []reading{
		{fieldName: "temperature", value: 28.5, ts: ts},
	}

	payloads, err := buildBatches(readings, schema, 1)
	require.NoError(t, err)
	require.Len(t, payloads, 1)

	var batch msgpackBatch
	require.NoError(t, msgpack.Unmarshal(payloads[0], &batch))
	assert.Equal(t, uint32(1), batch.SV)
	assert.Equal(t, ts.Unix(), batch.TS)
	assert.Equal(t, []float64{28.5}, batch.F)
	assert.Equal(t, []uint16{0}, batch.TD)
}

func TestBuildBatches_MultipleFields_TDCalculation(t *testing.T) {
	schema := makeSchema("temperature", "humidity")
	base := time.Unix(1700000000, 0).UTC()
	t1 := base
	t2 := base.Add(100 * time.Millisecond)

	readings := []reading{
		{fieldName: "temperature", value: 28.5, ts: t1},
		{fieldName: "humidity", value: 65.0, ts: t2},
	}

	payloads, err := buildBatches(readings, schema, 1)
	require.NoError(t, err)
	require.Len(t, payloads, 1)

	var batch msgpackBatch
	require.NoError(t, msgpack.Unmarshal(payloads[0], &batch))
	assert.Equal(t, []float64{28.5, 65.0}, batch.F)
	// temperature at offset 0, humidity at offset 100ms
	assert.Equal(t, []uint16{0, 100}, batch.TD)
}

func TestBuildBatches_SplitOnTDOverflow(t *testing.T) {
	schema := makeSchema("temperature")
	base := time.Unix(1700000000, 0).UTC()
	t1 := base
	// 66 seconds > 65535ms, so it should split into a new batch.
	t2 := base.Add(66 * time.Second)

	readings := []reading{
		{fieldName: "temperature", value: 20.0, ts: t1},
		{fieldName: "temperature", value: 21.0, ts: t2},
	}

	payloads, err := buildBatches(readings, schema, 1)
	require.NoError(t, err)
	require.Len(t, payloads, 2, "should split into two batches on TD overflow")

	var b1, b2 msgpackBatch
	require.NoError(t, msgpack.Unmarshal(payloads[0], &b1))
	require.NoError(t, msgpack.Unmarshal(payloads[1], &b2))

	assert.Equal(t, []float64{20.0}, b1.F)
	assert.Equal(t, []uint16{0}, b1.TD)

	assert.Equal(t, []float64{21.0}, b2.F)
	assert.Equal(t, []uint16{0}, b2.TD)
	assert.Equal(t, t2.Unix(), b2.TS)
}

func TestBuildBatches_EmptyReadings(t *testing.T) {
	schema := makeSchema("temperature")
	_, err := buildBatches([]reading{}, schema, 1)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no readings")
}

func TestBuildBatches_UnknownField(t *testing.T) {
	schema := makeSchema("temperature")
	readings := []reading{
		{fieldName: "pressure", value: 1013.0, ts: time.Now()},
	}
	_, err := buildBatches(readings, schema, 1)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown field")
}

func TestBuildBatches_EmptySchema(t *testing.T) {
	schema := channelSchema{
		nameToIndex:   map[string]uint8{},
		indexToName:   map[uint8]string{},
		schemaVersion: 1,
		orderedFields: nil,
	}
	readings := []reading{
		{fieldName: "temperature", value: 28.5, ts: time.Now()},
	}
	_, err := buildBatches(readings, schema, 1)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no fields")
}

func TestBuildBatches_SchemaVersionInPayload(t *testing.T) {
	schema := makeSchema("temperature")
	readings := []reading{
		{fieldName: "temperature", value: 28.5, ts: time.Now()},
	}

	payloads, err := buildBatches(readings, schema, 42)
	require.NoError(t, err)
	require.Len(t, payloads, 1)

	var batch msgpackBatch
	require.NoError(t, msgpack.Unmarshal(payloads[0], &batch))
	assert.Equal(t, uint32(42), batch.SV)
}

func TestBuildBatches_ExactlyAtTDLimit_NoSplit(t *testing.T) {
	schema := makeSchema("temperature")
	base := time.Unix(1700000000, 0).UTC()
	// 65535ms is the exact maximum — should NOT split.
	t2 := base.Add(65535 * time.Millisecond)

	readings := []reading{
		{fieldName: "temperature", value: 20.0, ts: base},
		{fieldName: "temperature", value: 21.0, ts: t2},
	}

	payloads, err := buildBatches(readings, schema, 1)
	require.NoError(t, err)
	// Both readings fit in one batch since the delta is exactly at the boundary.
	// The second reading overwrites the first (same field slot) — resulting in 1 batch.
	require.Len(t, payloads, 1)
}
