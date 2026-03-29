package http

import (
	"fmt"
	"time"

	"github.com/greenlab/ingestion/internal/application"
	"github.com/greenlab/ingestion/internal/domain"
)

// CompactBatch is the intermediate representation produced by compact-format parsers
// before field index resolution. All compact formats produce this struct.
type CompactBatch struct {
	// FieldValues maps field INDEX (uint8) → float64 value
	FieldValues   map[uint8]float64
	Timestamp     *time.Time // unix ms, nil = server-assigned
	TDOffsets     []uint16   // per-field delta ms; len 0 = all use batch timestamp
	SchemaVersion uint32     // mandatory in all compact formats
	DeviceID      string     // for binary frame DEVID validation only
}

// Deserializer parses raw bytes into a CompactBatch.
// Step 2 only — raw byte parsing. Steps 1/3/4/5 are shared in deserializeCompact.
type Deserializer interface {
	Parse(body []byte) (CompactBatch, error)
	ContentType() string
}

// deserializeCompact is the shared pipeline for all compact formats.
//
// Steps:
//  1. Body size cap — already applied by the caller before reaching here.
//  2. Format-specific Parse() call.
//  3. Schema version check — batch.SchemaVersion must match schema.SchemaVersion.
//     If schema.SchemaVersion == 0 (pre-TODO-028), skip the check.
//  4. Field index resolution — resolve each index in batch.FieldValues to a field name.
//  5. resolveTDOffsets — convert TDOffsets + batch.Timestamp to per-field timestamps.
//  6. Build []IngestInput (caller sets ChannelID and DeviceID afterwards).
func deserializeCompact(body []byte, schema domain.DeviceSchema, d Deserializer) ([]application.IngestInput, error) {
	// Step 2: parse
	batch, err := d.Parse(body)
	if err != nil {
		return nil, err
	}

	// Step 3: schema version check
	if schema.SchemaVersion != 0 && batch.SchemaVersion != 0 &&
		batch.SchemaVersion != schema.SchemaVersion {
		return nil, fmt.Errorf("%w: got %d, expected %d",
			domain.ErrSchemaMismatch, batch.SchemaVersion, schema.SchemaVersion)
	}

	// Step 4: field index resolution
	indexToField := make(map[uint8]domain.FieldEntry, len(schema.Fields))
	for _, f := range schema.Fields {
		indexToField[f.Index] = f
	}

	baseTS := time.Now().UTC()
	if batch.Timestamp != nil {
		baseTS = *batch.Timestamp
	}

	// Collect ordered field names for TD offset resolution.
	// We iterate in index order so TDOffsets are aligned.
	type resolvedField struct {
		name  string
		value float64
	}
	resolved := make([]resolvedField, 0, len(batch.FieldValues))
	// Sort by index for stable TDOffset alignment.
	for idx := uint8(1); idx <= 8; idx++ {
		val, ok := batch.FieldValues[idx]
		if !ok {
			continue
		}
		fe, known := indexToField[idx]
		if !known {
			return nil, fmt.Errorf("%w: index %d", domain.ErrUnknownFieldIndex, idx)
		}
		resolved = append(resolved, resolvedField{name: fe.Name, value: val})
	}

	// Step 5: resolve per-field timestamps.
	fieldNames := make([]string, len(resolved))
	for i, rf := range resolved {
		fieldNames[i] = rf.name
	}
	fieldTimestamps, err := resolveTDOffsets(baseTS, batch.TDOffsets, fieldNames)
	if err != nil {
		return nil, err
	}

	// Step 6: build IngestInput — group all fields into a single IngestInput.
	// Each field may have its own timestamp via fieldTimestamps.
	fields := make(map[string]float64, len(resolved))
	for _, rf := range resolved {
		fields[rf.name] = rf.value
	}

	return []application.IngestInput{
		{
			Fields:          fields,
			FieldTimestamps: fieldTimestamps,
			Timestamp:       batch.Timestamp,
		},
	}, nil
}
