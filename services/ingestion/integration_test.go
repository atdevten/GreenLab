//go:build integration

package integration_test

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"github.com/segmentio/kafka-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	kafkaTC "github.com/testcontainers/testcontainers-go/modules/kafka"
	tcredis "github.com/testcontainers/testcontainers-go/modules/redis"

	"github.com/greenlab/ingestion/internal/application"
	"github.com/greenlab/ingestion/internal/domain"
	kafkaInfra "github.com/greenlab/ingestion/internal/infrastructure/kafka"
	transporthttp "github.com/greenlab/ingestion/internal/transport/http"
)

const (
	testAPIKey    = "ts_testkey_integration"
	testChannelID = "550e8400-e29b-41d4-a716-446655440000"
	testDeviceID  = "device-integration-001"
	topicReadings = "raw.sensor.ingest"
)

// startKafka spins up a Kafka testcontainer and returns its broker address.
// The container is terminated via t.Cleanup.
func startKafka(t *testing.T) []string {
	t.Helper()
	ctx := context.Background()

	ctr, err := kafkaTC.Run(ctx, "confluentinc/confluent-local:7.5.0")
	require.NoError(t, err, "start kafka container")
	t.Cleanup(func() { _ = ctr.Terminate(ctx) })

	brokers, err := ctr.Brokers(ctx)
	require.NoError(t, err, "get kafka brokers")
	return brokers
}

// startRedis spins up a Redis testcontainer and returns a connected *redis.Client.
// The container is terminated via t.Cleanup.
func startRedis(t *testing.T) *redis.Client {
	t.Helper()
	ctx := context.Background()

	ctr, err := tcredis.Run(ctx, "redis:7-alpine")
	require.NoError(t, err, "start redis container")
	t.Cleanup(func() { _ = ctr.Terminate(ctx) })

	endpoint, err := ctr.Endpoint(ctx, "")
	require.NoError(t, err, "get redis endpoint")

	client := redis.NewClient(&redis.Options{Addr: endpoint})
	t.Cleanup(func() { client.Close() })

	require.Eventually(t, func() bool {
		return client.Ping(ctx).Err() == nil
	}, 30*time.Second, 200*time.Millisecond, "redis did not become ready")

	return client
}

// newTestRouter wires up the real stack: ReadingProducer → IngestService → Handler → NewRouter.
// testAPIKey is the only accepted key; any other triggers ErrDeviceNotFound.
func newTestRouter(t *testing.T, brokers []string, rdb *redis.Client) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)

	producer := kafkaInfra.NewReadingProducer(brokers)
	t.Cleanup(func() { _ = producer.Close() })

	svc := application.NewIngestService(producer, slog.Default(), 0 /* no max age in tests */)
	handler := transporthttp.NewHandler(svc, slog.Default())

	return transporthttp.NewRouter(handler, stubLookup, slog.Default(), rdb)
}

// newAuthOnlyRouter builds a router without a real Kafka producer — only the auth
// middleware is exercised. Safe to use for tests that expect a 401 before any
// publish logic is reached.
func newAuthOnlyRouter(t *testing.T, rdb *redis.Client) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)

	// noopPublisher satisfies EventPublisher but never touches Kafka.
	svc := application.NewIngestService(noopPublisher{}, slog.Default(), 0)
	handler := transporthttp.NewHandler(svc, slog.Default())

	return transporthttp.NewRouter(handler, stubLookup, slog.Default(), rdb)
}

// stubLookup accepts only testAPIKey; everything else returns ErrDeviceNotFound.
func stubLookup(_ context.Context, key, channelID string) (domain.DeviceSchema, error) {
	if key == testAPIKey {
		return domain.DeviceSchema{DeviceID: testDeviceID, ChannelID: channelID}, nil
	}
	return domain.DeviceSchema{}, domain.ErrDeviceNotFound
}

// noopPublisher is an EventPublisher that discards all readings.
type noopPublisher struct{}

func (noopPublisher) PublishReadings(_ context.Context, _ []*domain.Reading) error { return nil }

// readKafkaMessages reads exactly count messages from topic and unmarshals each into a map.
// Fails the test if count messages are not received within timeout.
func readKafkaMessages(t *testing.T, brokers []string, topic string, count int, timeout time.Duration) []map[string]interface{} {
	t.Helper()
	reader := kafka.NewReader(kafka.ReaderConfig{
		Brokers:   brokers,
		Topic:     topic,
		GroupID:   "integration-test-" + t.Name(),
		MinBytes:  1,
		MaxBytes:  1 << 20,
		MaxWait:   500 * time.Millisecond,
		StartOffset: kafka.FirstOffset,
	})
	t.Cleanup(func() { _ = reader.Close() })

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	var messages []map[string]interface{}
	for len(messages) < count {
		msg, err := reader.FetchMessage(ctx)
		require.NoError(t, err, "fetch kafka message")
		var payload map[string]interface{}
		require.NoError(t, json.Unmarshal(msg.Value, &payload), "unmarshal kafka message")
		messages = append(messages, payload)
		require.NoError(t, reader.CommitMessages(ctx, msg))
	}
	return messages
}

// ─── Tests ──────────────────────────────────────────────────────────────────

// TestIngest_ValidRequest_PublishesToKafka verifies the full HTTP → Kafka path:
// a valid POST with a real API key returns 201 and writes a message to the topic.
func TestIngest_ValidRequest_PublishesToKafka(t *testing.T) {
	brokers := startKafka(t)
	rdb := startRedis(t)

	router := newTestRouter(t, brokers, rdb)

	body, _ := json.Marshal(map[string]interface{}{
		"fields": map[string]float64{"temperature": 22.5},
	})
	req := httptest.NewRequest(http.MethodPost, "/v1/channels/"+testChannelID+"/data", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-Key", testAPIKey)

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code, "expected 201; body: %s", w.Body.String())

	msgs := readKafkaMessages(t, brokers, topicReadings, 1, 15*time.Second)
	require.Len(t, msgs, 1)

	reading, ok := msgs[0]["reading"].(map[string]interface{})
	require.True(t, ok, "reading field missing from kafka message")
	assert.Equal(t, testChannelID, reading["channel_id"])
	assert.Equal(t, testDeviceID, reading["device_id"])

	fields, ok := reading["fields"].(map[string]interface{})
	require.True(t, ok, "fields missing from reading")
	assert.InDelta(t, 22.5, fields["temperature"], 0.001)
}

// TestIngest_MissingAPIKey_Returns401 verifies that a request without an API key is rejected.
func TestIngest_MissingAPIKey_Returns401(t *testing.T) {
	rdb := startRedis(t)

	router := newAuthOnlyRouter(t, rdb)

	body, _ := json.Marshal(map[string]interface{}{
		"fields": map[string]float64{"temperature": 22.5},
	})
	req := httptest.NewRequest(http.MethodPost, "/v1/channels/"+testChannelID+"/data", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	// No X-API-Key header.

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code, "expected 401; body: %s", w.Body.String())
}

// TestIngest_InvalidAPIKey_Returns401 verifies that an unrecognised API key returns 401.
func TestIngest_InvalidAPIKey_Returns401(t *testing.T) {
	rdb := startRedis(t)

	router := newAuthOnlyRouter(t, rdb)

	body, _ := json.Marshal(map[string]interface{}{
		"fields": map[string]float64{"temperature": 22.5},
	})
	req := httptest.NewRequest(http.MethodPost, "/v1/channels/"+testChannelID+"/data", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-Key", "wrong-key")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code, "expected 401; body: %s", w.Body.String())
}

// TestBulkIngest_ValidRequest_PublishesToKafka verifies that a bulk POST publishes
// one Kafka message per reading entry.
func TestBulkIngest_ValidRequest_PublishesToKafka(t *testing.T) {
	brokers := startKafka(t)
	rdb := startRedis(t)

	router := newTestRouter(t, brokers, rdb)

	body, _ := json.Marshal(map[string]interface{}{
		"readings": []map[string]interface{}{
			{"fields": map[string]float64{"temp": 20.0}},
			{"fields": map[string]float64{"humidity": 60.0}},
		},
	})
	req := httptest.NewRequest(http.MethodPost, "/v1/channels/"+testChannelID+"/data/bulk", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-Key", testAPIKey)

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code, "expected 201; body: %s", w.Body.String())

	msgs := readKafkaMessages(t, brokers, topicReadings, 2, 15*time.Second)
	assert.Len(t, msgs, 2, "expected 2 kafka messages for 2 readings")
}
