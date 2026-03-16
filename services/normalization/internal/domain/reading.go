package domain

import "time"

// ReadingPayload is the telemetry data point extracted from a raw ingest event.
type ReadingPayload struct {
	ChannelID string             `json:"channel_id"`
	DeviceID  string             `json:"device_id"`
	Fields    map[string]float64 `json:"fields"`
	Tags      map[string]string  `json:"tags"`
	Timestamp time.Time          `json:"timestamp"`
}

// ReadingEvent is the envelope published by the ingestion service onto raw.sensor.ingest.
type ReadingEvent struct {
	ID          string         `json:"id"`
	Type        string         `json:"type"`
	PublishedAt time.Time      `json:"published_at"`
	Reading     ReadingPayload `json:"reading"`
}
