package video

import "errors"

var (
	ErrStreamNotFound     = errors.New("stream not found")
	ErrInvalidStreamID    = errors.New("invalid stream ID")
	ErrInvalidDeviceID    = errors.New("invalid device_id")
	ErrInvalidWorkspaceID = errors.New("invalid workspace_id")
	ErrNoRecording        = errors.New("no recording available for this stream")
	ErrInvalidProtocol    = errors.New("invalid stream protocol")
	ErrInvalidStatus      = errors.New("invalid stream status")
)
