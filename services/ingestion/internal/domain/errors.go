package domain

import "errors"

var (
	ErrInvalidChannelID = errors.New("channel_id must be a valid UUID")
	ErrEmptyFields      = errors.New("fields must not be empty")
	ErrCacheMiss        = errors.New("cache miss")
	ErrDeviceNotFound   = errors.New("device not found")
	ErrTimestampTooOld  = errors.New("timestamp is too far in the past")
	ErrTimestampFuture  = errors.New("timestamp is in the future")

	// Compact format errors
	ErrSchemaMismatch       = errors.New("schema_version mismatch")
	ErrMissingSchemaVersion = errors.New("missing_schema_version")
	ErrUnknownFieldIndex    = errors.New("unknown_field_index")
	ErrPayloadTooLarge      = errors.New("payload_too_large")
	ErrBodyReadError        = errors.New("body_read_error")
	ErrCRCMismatch          = errors.New("crc_mismatch")
	ErrDeviceIDMismatch     = errors.New("device_id_mismatch")
	ErrInvalidFrameLength   = errors.New("invalid_frame_length")
	ErrTSDeltaOverflow      = errors.New("timestamp_delta_overflow")
	ErrTSDeltaInvalid       = errors.New("timestamp_delta_invalid")
)
