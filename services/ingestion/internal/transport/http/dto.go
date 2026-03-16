package http

import "time"

// IngestRequest is the payload for a single telemetry write.
// ChannelID is intentionally absent — it is taken from the authenticated API key context,
// not from the request body, to prevent a device writing to a foreign channel.
type IngestRequest struct {
	Fields    map[string]float64 `json:"fields"    validate:"required"`
	Tags      map[string]string  `json:"tags"`
	Timestamp *time.Time         `json:"timestamp"`
}

// BulkIngestRequest contains a list of readings.
type BulkIngestRequest struct {
	Readings []IngestRequest `json:"readings" validate:"required"`
}

// IngestResponse acknowledges a successful write.
type IngestResponse struct {
	Accepted  int       `json:"accepted"`
	WrittenAt time.Time `json:"written_at"`
}
