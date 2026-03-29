package http

import (
	"fmt"
	"time"

	"github.com/greenlab/ingestion/internal/domain"
)

// resolveTDOffsets resolves per-field timestamp deltas (uint16 milliseconds since baseTS)
// into absolute per-field timestamps.
//
// Rules:
//   - If td is nil or empty: all fields use baseTS (no per-field timestamps)
//   - If len(td) != len(fieldNames): return ErrTSDeltaInvalid
//   - Returns map[fieldName]*time.Time; all values are non-nil
func resolveTDOffsets(baseTS time.Time, td []uint16, fieldNames []string) (map[string]*time.Time, error) {
	result := make(map[string]*time.Time, len(fieldNames))

	if len(td) == 0 {
		// All fields share baseTS.
		for _, name := range fieldNames {
			ts := baseTS
			result[name] = &ts
		}
		return result, nil
	}

	if len(td) != len(fieldNames) {
		return nil, fmt.Errorf("%w: %d td offsets for %d fields",
			domain.ErrTSDeltaInvalid, len(td), len(fieldNames))
	}

	for i, name := range fieldNames {
		ts := baseTS.Add(time.Duration(td[i]) * time.Millisecond)
		result[name] = &ts
	}
	return result, nil
}
