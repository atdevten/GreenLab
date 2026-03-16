package video

import (
	"time"

	"github.com/google/uuid"
)

// StreamStatus represents the lifecycle state of a video stream.
type StreamStatus string

const (
	StreamStatusPending  StreamStatus = "pending"
	StreamStatusLive     StreamStatus = "live"
	StreamStatusRecorded StreamStatus = "recorded"
	StreamStatusArchived StreamStatus = "archived"
)

// StreamProtocol is the streaming protocol used.
type StreamProtocol string

const (
	StreamProtocolRTSP   StreamProtocol = "rtsp"
	StreamProtocolHLS    StreamProtocol = "hls"
	StreamProtocolWebRTC StreamProtocol = "webrtc"
)

// Stream represents a video stream associated with a device.
type Stream struct {
	ID           uuid.UUID
	DeviceID     uuid.UUID
	WorkspaceID  uuid.UUID
	Name         string
	Description  string
	Protocol     StreamProtocol
	SourceURL    string
	StorageKey   string // S3 object key for recordings
	Status       StreamStatus
	ThumbnailURL string
	DurationSec  int
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// IsValid reports whether the protocol is a known value.
func (p StreamProtocol) IsValid() bool {
	switch p {
	case StreamProtocolRTSP, StreamProtocolHLS, StreamProtocolWebRTC:
		return true
	}
	return false
}

// IsValid reports whether the status is a known value.
func (s StreamStatus) IsValid() bool {
	switch s {
	case StreamStatusPending, StreamStatusLive, StreamStatusRecorded, StreamStatusArchived:
		return true
	}
	return false
}

// NewStream creates a new Stream entity.
func NewStream(deviceID, workspaceID uuid.UUID, name, description string, protocol StreamProtocol, sourceURL string) *Stream {
	now := time.Now().UTC()
	return &Stream{
		ID:          uuid.New(),
		DeviceID:    deviceID,
		WorkspaceID: workspaceID,
		Name:        name,
		Description: description,
		Protocol:    protocol,
		SourceURL:   sourceURL,
		Status:      StreamStatusPending,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
}
