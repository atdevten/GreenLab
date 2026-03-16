package query

import "time"

// QueryRequest defines parameters for a telemetry query.
type QueryRequest struct {
	ChannelID string
	FieldName string
	Start     time.Time
	End       time.Time
	Limit     int
	Aggregate string // mean, sum, count, last, first
	Window    string // e.g., "1m", "1h", "1d"
}

// DataPoint is a single time-series data point.
type DataPoint struct {
	Timestamp time.Time `json:"timestamp"`
	Value     float64   `json:"value"`
	Field     string    `json:"field"`
}

// QueryResult holds the results of a telemetry query.
type QueryResult struct {
	ChannelID  string      `json:"channel_id"`
	FieldName  string      `json:"field_name"`
	DataPoints []DataPoint `json:"data_points"`
	Count      int         `json:"count"`
	Start      time.Time   `json:"start"`
	End        time.Time   `json:"end"`
}

// LatestReading is the most recent value for a field.
type LatestReading struct {
	ChannelID string    `json:"channel_id"`
	FieldName string    `json:"field_name"`
	Value     float64   `json:"value"`
	Timestamp time.Time `json:"timestamp"`
}
