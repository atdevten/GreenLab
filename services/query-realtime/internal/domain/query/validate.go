package query

import (
	"regexp"

	"github.com/google/uuid"
)

var (
	validAggregates = map[string]bool{
		"mean": true, "sum": true, "count": true,
		"last": true, "first": true, "min": true, "max": true,
	}
	// validWindowRe accepts e.g. "1m", "30s", "2h", "7d".
	validWindowRe = regexp.MustCompile(`^\d+[smhd]$`)
	// validFieldNameRe allows letters, digits, dashes, and underscores only.
	validFieldNameRe = regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)
)

// ValidateQueryRequest validates every user-supplied field that will be
// interpolated into a Flux query string. This must be called by the
// application layer before passing a request to any Reader implementation,
// regardless of which transport (HTTP, gRPC, …) originated the call.
func ValidateQueryRequest(req *QueryRequest) error {
	if _, err := uuid.Parse(req.ChannelID); err != nil {
		return ErrInvalidChannelID
	}
	if req.FieldName != "" && !validFieldNameRe.MatchString(req.FieldName) {
		return ErrInvalidFieldName
	}
	if req.Aggregate != "" && !validAggregates[req.Aggregate] {
		return ErrInvalidAggregate
	}
	if req.Window != "" && !validWindowRe.MatchString(req.Window) {
		return ErrInvalidWindow
	}
	return nil
}
