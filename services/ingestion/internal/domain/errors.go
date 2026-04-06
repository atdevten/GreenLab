package domain

import (
	"errors"
	"fmt"
)

var (
	ErrInvalidChannelID = errors.New("channel_id must be a valid UUID")
	ErrEmptyFields      = errors.New("fields must not be empty")
	ErrCacheMiss        = errors.New("cache miss")
	ErrDeviceNotFound   = errors.New("device not found")
	ErrTimestampTooOld              = errors.New("timestamp is too far in the past")
	ErrTimestampFuture              = errors.New("timestamp is in the future")
	ErrTimestampOutOfReplayWindow   = errors.New("timestamp_out_of_replay_window")

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

// SchemaMismatchError carries the channel's current schema version for the 409 response.
// It is returned by compact-format deserializers when the device's schema_version
// does not match the server's current schema version for the channel.
type SchemaMismatchError struct {
	CurrentVersion uint32
	ChannelID      string
}

func (e *SchemaMismatchError) Error() string {
	return fmt.Sprintf("schema_version mismatch: server has version %d", e.CurrentVersion)
}
