package query

import "errors"

var (
	ErrInvalidChannelID = errors.New("channel_id must be a valid UUID")
	ErrInvalidTimeRange = errors.New("start must be before end")
	ErrNoDataFound      = errors.New("no data found")
	ErrInvalidFieldName = errors.New("field name must contain only letters, digits, dashes, or underscores")
	ErrInvalidAggregate = errors.New("aggregate must be one of: mean, sum, count, last, first, min, max")
	ErrInvalidWindow    = errors.New("window must be a positive integer followed by s, m, h, or d (e.g. 1m, 5m, 1h)")
)
