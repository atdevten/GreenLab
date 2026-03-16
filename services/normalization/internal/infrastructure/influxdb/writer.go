package influxdb

import (
	"context"
	"fmt"

	influxdb2 "github.com/influxdata/influxdb-client-go/v2"
	"github.com/influxdata/influxdb-client-go/v2/api"
	"github.com/influxdata/influxdb-client-go/v2/api/write"
	"github.com/greenlab/normalization/internal/domain"
)

// Writer writes normalized telemetry readings to InfluxDB.
type Writer struct {
	client   influxdb2.Client
	writeAPI api.WriteAPIBlocking
}

// Config holds InfluxDB connection config.
type Config struct {
	URL    string
	Token  string
	Org    string
	Bucket string
}

// NewWriter creates a new InfluxDB writer.
func NewWriter(cfg Config) *Writer {
	client := influxdb2.NewClient(cfg.URL, cfg.Token)
	return &Writer{
		client:   client,
		writeAPI: client.WriteAPIBlocking(cfg.Org, cfg.Bucket),
	}
}

// Close releases the InfluxDB client.
func (w *Writer) Close() {
	w.client.Close()
}

// Write writes a single reading payload to InfluxDB.
func (w *Writer) Write(ctx context.Context, r *domain.ReadingPayload) error {
	if err := w.writeAPI.WritePoint(ctx, buildPoint(r)); err != nil {
		return fmt.Errorf("Writer.Write: %w", err)
	}
	return nil
}

func buildPoint(r *domain.ReadingPayload) *write.Point {
	p := influxdb2.NewPointWithMeasurement("telemetry").
		AddTag("channel_id", r.ChannelID).
		AddTag("device_id", r.DeviceID).
		SetTime(r.Timestamp)

	for k, v := range r.Tags {
		p = p.AddTag(k, v)
	}
	for k, v := range r.Fields {
		p = p.AddField(k, v)
	}
	return p
}
