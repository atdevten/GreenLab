//go:build integration

package influxdb

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	influxdb2 "github.com/influxdata/influxdb-client-go/v2"
	"github.com/greenlab/normalization/internal/domain"
)

func getEnv(key, defaultVal string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultVal
}

func TestWriter_WriteIdempotency(t *testing.T) {
	influxURL := os.Getenv("INFLUXDB_URL")
	if influxURL == "" {
		t.Skipf("INFLUXDB_URL not set — skipping integration test")
	}

	token := getEnv("INFLUXDB_TOKEN", "my-super-secret-token")
	org := getEnv("INFLUXDB_ORG", "greenlab")
	bucket := getEnv("INFLUXDB_BUCKET", "telemetry")

	cfg := Config{
		URL:    influxURL,
		Token:  token,
		Org:    org,
		Bucket: bucket,
	}

	w := NewWriter(cfg)
	defer w.Close()

	// Use a fixed, truncated timestamp so both writes land on the same line-protocol point.
	// InfluxDB deduplicates points with identical measurement + tag set + timestamp.
	ts := time.Now().UTC().Truncate(time.Second)

	payload := &domain.ReadingPayload{
		ChannelID: "ch-idempotency-test",
		DeviceID:  "dev-idempotency-test",
		Fields:    map[string]float64{"temperature": 23.5},
		Tags:      map[string]string{"test": "idempotency"},
		Timestamp: ts,
	}

	ctx := context.Background()

	// Write the same payload twice to simulate an at-least-once retry scenario.
	if err := w.Write(ctx, payload); err != nil {
		t.Fatalf("first Write failed: %v", err)
	}
	if err := w.Write(ctx, payload); err != nil {
		t.Fatalf("second Write failed: %v", err)
	}

	// Allow InfluxDB a moment to flush the writes before querying.
	time.Sleep(500 * time.Millisecond)

	// Query for records with the exact channel_id tag and a 1-second window around the timestamp.
	queryClient := influxdb2.NewClient(influxURL, token)
	defer queryClient.Close()

	queryAPI := queryClient.QueryAPI(org)

	// Narrow time range: [ts-1s, ts+1s] ensures we isolate only the test point.
	start := ts.Add(-time.Second).Format(time.RFC3339)
	stop := ts.Add(time.Second).Format(time.RFC3339)

	flux := fmt.Sprintf(`
from(bucket: "%s")
  |> range(start: %s, stop: %s)
  |> filter(fn: (r) => r._measurement == "telemetry")
  |> filter(fn: (r) => r.channel_id == "%s")
  |> filter(fn: (r) => r.device_id == "%s")
  |> filter(fn: (r) => r._field == "temperature")
  |> count()
`,
		bucket, start, stop, payload.ChannelID, payload.DeviceID,
	)

	result, err := queryAPI.Query(ctx, flux)
	if err != nil {
		t.Fatalf("query failed: %v", err)
	}
	defer result.Close()

	var count int64
	for result.Next() {
		v := result.Record().Value()
		switch n := v.(type) {
		case int64:
			count = n
		case float64:
			count = int64(n)
		default:
			t.Fatalf("unexpected count value type %T: %v", v, v)
		}
	}
	if err := result.Err(); err != nil {
		t.Fatalf("query result error: %v", err)
	}

	if count != 1 {
		t.Errorf("idempotency check failed: expected 1 record, got %d", count)
	}
}
