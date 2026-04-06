package kafka

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/segmentio/kafka-go"
	"go.opentelemetry.io/otel/trace"
	"github.com/greenlab/ingestion/internal/domain"
)

const topicReadings = "raw.sensor.ingest"

// ReadingProducer publishes reading events to Kafka.
type ReadingProducer struct {
	writer *kafka.Writer
}

// NewReadingProducer creates a new ReadingProducer.
func NewReadingProducer(brokers []string) *ReadingProducer {
	return &ReadingProducer{
		writer: &kafka.Writer{
			Addr:         kafka.TCP(brokers...),
			Topic:        topicReadings,
			Balancer:     &kafka.Hash{},
			RequiredAcks: kafka.RequireAll, // wait for all in-sync replicas to prevent data loss
			BatchSize:    500,
			BatchTimeout: 5 * time.Millisecond,
			Compression:  kafka.Snappy,
		},
	}
}

// Close closes the producer writer.
func (p *ReadingProducer) Close() error {
	return p.writer.Close()
}

// readingPayload is the JSON-serialisable representation of a domain.Reading.
type readingPayload struct {
	ChannelID string             `json:"channel_id"`
	DeviceID  string             `json:"device_id"`
	Fields    map[string]float64 `json:"fields"`
	Tags      map[string]string  `json:"tags"`
	Timestamp time.Time          `json:"timestamp"`
}

type readingEvent struct {
	ID          string         `json:"id"`
	Type        string         `json:"type"`
	PublishedAt time.Time      `json:"published_at"` // when the event entered Kafka, not the measurement time
	Reading     readingPayload `json:"reading"`
}

// buildMessages serializes a batch of readings into Kafka messages.
// If replay is true, a "replay: true" header is added to each message.
// If the context carries a valid OTel span, a W3C traceparent header is injected
// so downstream consumers can continue the trace.
func buildMessages(ctx context.Context, readings []*domain.Reading, replay bool) ([]kafka.Message, error) {
	msgs := make([]kafka.Message, 0, len(readings))
	for _, r := range readings {
		evt := readingEvent{
			ID:          uuid.New().String(),
			Type:        "reading.ingested",
			PublishedAt: time.Now().UTC(),
			Reading: readingPayload{
				ChannelID: r.ChannelID,
				DeviceID:  r.DeviceID,
				Fields:    r.Fields,
				Tags:      r.Tags,
				Timestamp: r.Timestamp,
			},
		}
		b, err := json.Marshal(evt)
		if err != nil {
			return nil, fmt.Errorf("marshal reading: %w", err)
		}
		msg := kafka.Message{
			Key:   []byte(r.ChannelID),
			Value: b,
		}

		var headers []kafka.Header
		if replay {
			headers = append(headers, kafka.Header{Key: "replay", Value: []byte("true")})
		}
		if span := trace.SpanFromContext(ctx); span.SpanContext().IsValid() {
			sc := span.SpanContext()
			traceparent := fmt.Sprintf("00-%s-%s-01", sc.TraceID().String(), sc.SpanID().String())
			headers = append(headers, kafka.Header{Key: "traceparent", Value: []byte(traceparent)})
		}
		if len(headers) > 0 {
			msg.Headers = headers
		}

		msgs = append(msgs, msg)
	}
	return msgs, nil
}

// PublishReadings sends a batch of reading events.
func (p *ReadingProducer) PublishReadings(ctx context.Context, readings []*domain.Reading) error {
	msgs, err := buildMessages(ctx, readings, false)
	if err != nil {
		return fmt.Errorf("ReadingProducer.PublishReadings: %w", err)
	}
	if len(msgs) == 0 {
		return nil
	}
	if err := p.writer.WriteMessages(ctx, msgs...); err != nil {
		return fmt.Errorf("ReadingProducer.PublishReadings: %w", err)
	}
	return nil
}

// PublishReplayReadings sends a batch of replay reading events with a "replay: true" header.
func (p *ReadingProducer) PublishReplayReadings(ctx context.Context, readings []*domain.Reading) error {
	msgs, err := buildMessages(ctx, readings, true)
	if err != nil {
		return fmt.Errorf("ReadingProducer.PublishReplayReadings: %w", err)
	}
	if len(msgs) == 0 {
		return nil
	}
	if err := p.writer.WriteMessages(ctx, msgs...); err != nil {
		return fmt.Errorf("ReadingProducer.PublishReplayReadings: %w", err)
	}
	return nil
}
