package http

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/greenlab/ingestion/internal/domain"
)

// ojsonPayload is the wire format for optimized JSON compact messages.
// Example: {"f":[42.5,28.1],"t":1741426620000,"td":[0,180],"sv":1}
type ojsonPayload struct {
	F  []float64 `json:"f"`           // positional field values (index 1, 2, 3...)
	T  *int64    `json:"t,omitempty"` // unix milliseconds
	TD []uint16  `json:"td,omitempty"`
	SV uint32    `json:"sv"`
}

type ojsonDeserializer struct{}

func newOJSONDeserializer() *ojsonDeserializer { return &ojsonDeserializer{} }

func (d *ojsonDeserializer) ContentType() string { return ctOJSON }

func (d *ojsonDeserializer) Parse(body []byte) (CompactBatch, error) {
	var p ojsonPayload
	if err := json.Unmarshal(body, &p); err != nil {
		return CompactBatch{}, fmt.Errorf("ojson: parse error: %w", err)
	}
	if len(p.F) == 0 {
		return CompactBatch{}, fmt.Errorf("ojson: %w: field \"f\" is required and must not be empty", domain.ErrEmptyFields)
	}

	// Map positional array index → field index (1-based).
	fieldValues := make(map[uint8]float64, len(p.F))
	for i, v := range p.F {
		fieldValues[uint8(i+1)] = v //nolint:gosec // i is bounded by len(p.F) ≤ 8
	}

	var ts *time.Time
	if p.T != nil {
		t := time.UnixMilli(*p.T).UTC()
		ts = &t
	}

	return CompactBatch{
		FieldValues:   fieldValues,
		Timestamp:     ts,
		TDOffsets:     p.TD,
		SchemaVersion: p.SV,
	}, nil
}
