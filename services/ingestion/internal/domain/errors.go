package domain

import "errors"

var (
	ErrInvalidChannelID = errors.New("channel_id must be a valid UUID")
	ErrEmptyFields      = errors.New("fields must not be empty")
	ErrCacheMiss        = errors.New("cache miss")
	ErrDeviceNotFound   = errors.New("device not found")
	ErrTimestampTooOld  = errors.New("timestamp is too far in the past")
	ErrTimestampFuture  = errors.New("timestamp is in the future")
)
