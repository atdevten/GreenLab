package channel

import "errors"

var (
	ErrChannelNotFound   = errors.New("channel not found")
	ErrInvalidName       = errors.New("name must not be empty")
	ErrInvalidVisibility = errors.New("visibility must be 'public' or 'private'")
	ErrInvalidRetention  = errors.New("retention_days must be between 1 and 365")
)
