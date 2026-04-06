package http

import (
	"encoding/json"
	"time"
)

// IngestRequest is the payload for a single telemetry write.
// ChannelID is intentionally absent — it is taken from the authenticated API key context,
// not from the request body, to prevent a device writing to a foreign channel.
type IngestRequest struct {
	Fields          map[string]float64    `json:"fields"            validate:"required"`
	FieldTimestamps map[string]*time.Time `json:"field_timestamps"`
	Tags            map[string]string     `json:"tags"`
	Timestamp       *time.Time            `json:"timestamp"`
	Data            json.RawMessage       `json:"data"` // optional opaque field for clients to include arbitrary JSON; not processed by the service
}

// BulkIngestRequest contains a list of readings.
type BulkIngestRequest struct {
	Readings []IngestRequest `json:"readings" validate:"required,max=1000"`
}

// IngestResponse acknowledges a successful write.
type IngestResponse struct {
	Accepted  int       `json:"accepted"`
	WrittenAt time.Time `json:"written_at"`
	ChannelID string    `json:"channel_id"`
	RequestID string    `json:"request_id"`
}

// ReplayReadingRequest is a single reading in a replay batch.
type ReplayReadingRequest struct {
	Fields    map[string]float64 `json:"fields"     validate:"required"`
	Timestamp time.Time          `json:"timestamp"`
}

// ReplayRequest contains a batch of replay readings.
type ReplayRequest struct {
	Readings []ReplayReadingRequest `json:"readings" validate:"required,min=1,max=1000,dive"`
}

// ReplayResponse acknowledges a replay request.
// QueuedForRetry is non-zero when Kafka was unavailable and readings were
// written to the DLQ instead of published immediately.
type ReplayResponse struct {
	Accepted       int    `json:"accepted"`
	QueuedForRetry int    `json:"queued_for_retry"`
	ChannelID      string `json:"channel_id"`
	RequestID      string `json:"request_id"`
}
