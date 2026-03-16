package influxdb

import (
	"context"
	"fmt"
	"time"

	influxdb2 "github.com/influxdata/influxdb-client-go/v2"
	"github.com/influxdata/influxdb-client-go/v2/api"
	"github.com/greenlab/query-realtime/internal/domain/query"
)

// Reader reads telemetry data from InfluxDB.
type Reader struct {
	client   influxdb2.Client
	queryAPI api.QueryAPI
	org      string
	bucket   string
}

// Config holds InfluxDB connection parameters.
type Config struct {
	URL    string
	Token  string
	Org    string
	Bucket string
}

// NewReader creates a new InfluxDB reader.
func NewReader(cfg Config) *Reader {
	client := influxdb2.NewClient(cfg.URL, cfg.Token)
	return &Reader{
		client:   client,
		queryAPI: client.QueryAPI(cfg.Org),
		org:      cfg.Org,
		bucket:   cfg.Bucket,
	}
}

// Close releases the client.
func (r *Reader) Close() {
	r.client.Close()
}

// Query executes a Flux query against InfluxDB.
func (r *Reader) Query(ctx context.Context, req *query.QueryRequest) (*query.QueryResult, error) {
	flux := r.buildFlux(req)

	result, err := r.queryAPI.Query(ctx, flux)
	if err != nil {
		return nil, fmt.Errorf("influxdb query: %w", err)
	}
	defer result.Close()

	qr := &query.QueryResult{
		ChannelID: req.ChannelID,
		FieldName: req.FieldName,
		Start:     req.Start,
		End:       req.End,
	}

	for result.Next() {
		rec := result.Record()
		dp := query.DataPoint{
			Timestamp: rec.Time(),
			Field:     req.FieldName,
		}
		if v, ok := rec.Value().(float64); ok {
			dp.Value = v
		}
		qr.DataPoints = append(qr.DataPoints, dp)
	}

	if err := result.Err(); err != nil {
		return nil, fmt.Errorf("influxdb result error: %w", err)
	}

	qr.Count = len(qr.DataPoints)
	return qr, nil
}

// QueryLatest returns the most recent value for a field.
func (r *Reader) QueryLatest(ctx context.Context, channelID, fieldName string) (*query.LatestReading, error) {
	flux := fmt.Sprintf(`
from(bucket: "%s")
  |> range(start: -30d)
  |> filter(fn: (r) => r["_measurement"] == "telemetry")
  |> filter(fn: (r) => r["channel_id"] == "%s")
  |> filter(fn: (r) => r["_field"] == "%s")
  |> last()
`, r.bucket, channelID, fieldName)

	result, err := r.queryAPI.Query(ctx, flux)
	if err != nil {
		return nil, fmt.Errorf("influxdb latest query: %w", err)
	}
	defer result.Close()

	for result.Next() {
		rec := result.Record()
		lr := &query.LatestReading{
			ChannelID: channelID,
			FieldName: fieldName,
			Timestamp: rec.Time(),
		}
		if v, ok := rec.Value().(float64); ok {
			lr.Value = v
		}
		// Check streaming error before returning the record.
		if err := result.Err(); err != nil {
			return nil, fmt.Errorf("influxdb latest result error: %w", err)
		}
		return lr, nil
	}

	if err := result.Err(); err != nil {
		return nil, fmt.Errorf("influxdb latest result error: %w", err)
	}

	return nil, query.ErrNoDataFound
}

func (r *Reader) buildFlux(req *query.QueryRequest) string {
	fieldFilter := ""
	if req.FieldName != "" {
		fieldFilter = fmt.Sprintf(`|> filter(fn: (r) => r["_field"] == "%s")`, req.FieldName)
	}

	aggregation := ""
	if req.Aggregate != "" && req.Window != "" {
		aggregation = fmt.Sprintf(`|> aggregateWindow(every: %s, fn: %s, createEmpty: false)`, req.Window, req.Aggregate)
	}

	limit := ""
	if req.Limit > 0 {
		limit = fmt.Sprintf(`|> limit(n: %d)`, req.Limit)
	}

	return fmt.Sprintf(`
from(bucket: "%s")
  |> range(start: %s, stop: %s)
  |> filter(fn: (r) => r["_measurement"] == "telemetry")
  |> filter(fn: (r) => r["channel_id"] == "%s")
  %s
  %s
  %s
`,
		r.bucket,
		req.Start.UTC().Format(time.RFC3339),
		req.End.UTC().Format(time.RFC3339),
		req.ChannelID,
		fieldFilter,
		aggregation,
		limit,
	)
}
