package http

import "time"

// QueryParams are the query-string parameters for the /query endpoint.
type QueryParams struct {
	ChannelID string `form:"channel_id" validate:"required"`
	FieldName string `form:"field"`
	Start     string `form:"start"`
	End       string `form:"end"`
	Limit     int    `form:"limit"`
	Aggregate string `form:"aggregate"`
	Window    string `form:"window"`
}

type DataPointResponse struct {
	Timestamp time.Time `json:"timestamp"`
	Value     float64   `json:"value"`
	Field     string    `json:"field"`
}

type QueryResponse struct {
	ChannelID  string              `json:"channel_id"`
	FieldName  string              `json:"field_name"`
	DataPoints []DataPointResponse `json:"data_points"`
	Count      int                 `json:"count"`
	Start      time.Time           `json:"start"`
	End        time.Time           `json:"end"`
}

type LatestReadingResponse struct {
	ChannelID string    `json:"channel_id"`
	FieldName string    `json:"field_name"`
	Value     float64   `json:"value"`
	Timestamp time.Time `json:"timestamp"`
}
