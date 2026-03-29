package http

import (
	"fmt"
	"time"

	"github.com/vmihailenco/msgpack/v5"
	"github.com/greenlab/ingestion/internal/domain"
)

// msgpackPayload mirrors ojsonPayload but decoded from MessagePack.
// Map keys use the same compact short names: "f", "t", "td", "sv".
type msgpackPayload struct {
	F  []float64 `msgpack:"f"`
	T  *int64    `msgpack:"t"`
	TD []uint16  `msgpack:"td"`
	SV uint32    `msgpack:"sv"`
}

type msgpackDeserializer struct{}

func newMsgPackDeserializer() *msgpackDeserializer { return &msgpackDeserializer{} }

func (d *msgpackDeserializer) ContentType() string { return ctMsgPack }

func (d *msgpackDeserializer) Parse(body []byte) (CompactBatch, error) {
	var p msgpackPayload
	if err := msgpack.Unmarshal(body, &p); err != nil {
		return CompactBatch{}, fmt.Errorf("msgpack: parse error: %w", err)
	}
	if len(p.F) == 0 {
		return CompactBatch{}, fmt.Errorf("msgpack: %w: field \"f\" is required and must not be empty", domain.ErrEmptyFields)
	}

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
