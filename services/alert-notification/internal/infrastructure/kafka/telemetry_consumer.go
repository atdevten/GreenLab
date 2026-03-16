package kafka

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"time"

	"github.com/segmentio/kafka-go"
	"github.com/greenlab/alert-notification/internal/application"
)

// Evaluator is the local interface for processing incoming telemetry events.
type Evaluator interface {
	Evaluate(ctx context.Context, evt application.TelemetryEvent)
}

// TelemetryConsumer consumes telemetry readings and feeds them to the rule engine.
type TelemetryConsumer struct {
	reader *kafka.Reader
	engine Evaluator
	log    *slog.Logger
}

// NewTelemetryConsumer creates a new TelemetryConsumer.
func NewTelemetryConsumer(brokers []string, groupID string, engine Evaluator, log *slog.Logger) *TelemetryConsumer {
	reader := kafka.NewReader(kafka.ReaderConfig{
		Brokers:     brokers,
		Topic:       "telemetry.readings",
		GroupID:     groupID,
		MinBytes:    1,
		MaxBytes:    10e6,
		MaxWait:     100 * time.Millisecond,
		StartOffset: kafka.FirstOffset,
	})
	return &TelemetryConsumer{reader: reader, engine: engine, log: log}
}

type telemetryReadingPayload struct {
	ChannelID string             `json:"channel_id"`
	DeviceID  string             `json:"device_id"`
	Fields    map[string]float64 `json:"fields"`
	Timestamp time.Time          `json:"timestamp"`
}

// telemetryReadingEnvelope matches the envelope format written by the ingestion producer.
type telemetryReadingEnvelope struct {
	ID          string                  `json:"id"`
	Type        string                  `json:"type"`
	PublishedAt time.Time               `json:"published_at"`
	Reading     telemetryReadingPayload `json:"reading"`
}

// Start begins consuming messages until ctx is cancelled.
func (c *TelemetryConsumer) Start(ctx context.Context) error {
	c.log.Info("alert telemetry kafka consumer started")
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		msg, err := c.reader.FetchMessage(ctx)
		if err != nil {
			if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
				return ctx.Err()
			}
			c.log.Error("fetch message", "error", err)
			continue
		}

		var env telemetryReadingEnvelope
		if err := json.Unmarshal(msg.Value, &env); err != nil {
			c.log.Warn("unmarshal telemetry envelope — skipping",
				"error", err,
				"offset", msg.Offset,
				"partition", msg.Partition,
			)
			if err := c.reader.CommitMessages(ctx, msg); err != nil {
				c.log.Error("commit unprocessable message",
					"error", err,
					"offset", msg.Offset,
					"partition", msg.Partition,
				)
			}
			continue
		}

		c.engine.Evaluate(ctx, application.TelemetryEvent{
			ChannelID: env.Reading.ChannelID,
			DeviceID:  env.Reading.DeviceID,
			Fields:    env.Reading.Fields,
			Timestamp: env.Reading.Timestamp,
		})

		if err := c.reader.CommitMessages(ctx, msg); err != nil {
			c.log.Error("commit message",
				"error", err,
				"offset", msg.Offset,
				"partition", msg.Partition,
			)
		}
	}
}

// Close closes the Kafka reader.
func (c *TelemetryConsumer) Close() error {
	return c.reader.Close()
}
