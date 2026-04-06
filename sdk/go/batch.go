package sdk

import (
	"fmt"
	"time"

	"github.com/vmihailenco/msgpack/v5"
)

const maxTDMillis = 65535

// reading represents a single field measurement at a point in time.
type reading struct {
	fieldName string
	value     float64
	ts        time.Time
}

// msgpackBatch is the wire-format struct for a batch payload.
// Field names use compact short keys matching the server's compact format.
type msgpackBatch struct {
	SV uint32    `msgpack:"sv"`
	TS int64     `msgpack:"ts"`
	F  []float64 `msgpack:"f"`
	TD []uint16  `msgpack:"td"`
}

// buildBatches partitions readings into one or more msgpackBatch values.
//
// Readings are grouped by their schema-ordered field position. The base
// timestamp is the timestamp of the first reading in each group. If any
// reading's delta from the base exceeds 65535ms, a new batch is started
// at that point.
//
// The returned [][]byte contains the MessagePack-encoded payload for each
// batch, ready to be optionally compressed and sent.
func buildBatches(readings []reading, schema channelSchema, schemaVersion uint32) ([][]byte, error) {
	if len(readings) == 0 {
		return nil, fmt.Errorf("sdk: no readings to send")
	}

	// Determine the number of fields from the schema.
	numFields := len(schema.orderedFields)
	if numFields == 0 {
		return nil, fmt.Errorf("sdk: schema has no fields")
	}

	// Build a lookup: fieldName → position in orderedFields (0-based).
	// The "f" array is positional, matching the server's ordered field list.
	fieldPos := make(map[string]int, numFields)
	for i, fe := range schema.orderedFields {
		fieldPos[fe.Name] = i
	}

	// Validate that all readings map to known fields.
	for _, r := range readings {
		if _, ok := fieldPos[r.fieldName]; !ok {
			return nil, fmt.Errorf("sdk: unknown field %q", r.fieldName)
		}
	}

	var (
		result    [][]byte
		baseTS    = readings[0].ts
		fVals     = make([]float64, numFields)
		tdVals    = make([]uint16, numFields)
		hasValue  = make([]bool, numFields)
		batchUsed bool
	)

	flushBatch := func() error {
		// Only include slots that have values.
		f := make([]float64, 0, numFields)
		td := make([]uint16, 0, numFields)
		for i := 0; i < numFields; i++ {
			if hasValue[i] {
				f = append(f, fVals[i])
				td = append(td, tdVals[i])
			}
		}
		if len(f) == 0 {
			return nil
		}
		batch := msgpackBatch{
			SV: schemaVersion,
			TS: baseTS.Unix(),
			F:  f,
			TD: td,
		}
		encoded, err := msgpack.Marshal(batch)
		if err != nil {
			return fmt.Errorf("sdk: encode batch: %w", err)
		}
		result = append(result, encoded)
		return nil
	}

	resetBatch := func(newBase time.Time) {
		baseTS = newBase
		fVals = make([]float64, numFields)
		tdVals = make([]uint16, numFields)
		hasValue = make([]bool, numFields)
		batchUsed = false
	}

	for _, r := range readings {
		deltaMs := r.ts.Sub(baseTS).Milliseconds()
		if deltaMs < 0 {
			deltaMs = 0
		}

		// If delta exceeds uint16 range, flush current batch and start a new one.
		if deltaMs > maxTDMillis {
			if batchUsed {
				if err := flushBatch(); err != nil {
					return nil, err
				}
			}
			resetBatch(r.ts)
			deltaMs = 0
		}

		pos := fieldPos[r.fieldName]
		fVals[pos] = r.value
		//nolint:gosec // deltaMs is bounded by maxTDMillis = 65535 which fits uint16
		tdVals[pos] = uint16(deltaMs)
		hasValue[pos] = true
		batchUsed = true
	}

	// Flush remaining readings.
	if batchUsed {
		if err := flushBatch(); err != nil {
			return nil, err
		}
	}

	return result, nil
}
