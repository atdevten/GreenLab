package kafka

import (
	"context"
	"encoding/json"
	"log/slog"
	"time"

	"github.com/segmentio/kafka-go"
	"github.com/greenlab/query-realtime/internal/domain/realtime"
)

const topicReadings = "telemetry.readings"

// Broadcaster is the interface for broadcasting push messages to subscribers.
type Broadcaster interface {
	Broadcast(msg *realtime.PushMessage)
}

// ReadingConsumer consumes telemetry events and broadcasts to the hub.
type ReadingConsumer struct {
	reader *kafka.Reader
	hub    Broadcaster
	log    *slog.Logger
}

// NewReadingConsumer creates a new Kafka consumer for telemetry readings.
func NewReadingConsumer(brokers []string, groupID string, hub Broadcaster, log *slog.Logger) *ReadingConsumer {
	reader := kafka.NewReader(kafka.ReaderConfig{
		Brokers:     brokers,
		Topic:       topicReadings,
		GroupID:     groupID,
		MinBytes:    1,
		MaxBytes:    10e6,
		MaxWait:     100 * time.Millisecond,
		StartOffset: kafka.LastOffset,
	})
	return &ReadingConsumer{reader: reader, hub: hub, log: log}
}

type readingEvent struct {
	ID          string         `json:"id"`
	Type        string         `json:"type"`
	PublishedAt time.Time      `json:"published_at"`
	Reading     readingPayload `json:"reading"`
}

type readingPayload struct {
	ChannelID string             `json:"channel_id"`
	DeviceID  string             `json:"device_id"`
	Fields    map[string]float64 `json:"fields"`
	Timestamp time.Time          `json:"timestamp"`
}

// Start begins consuming messages until context is cancelled.
func (c *ReadingConsumer) Start(ctx context.Context) error {
	c.log.Info("realtime kafka consumer started", "topic", topicReadings)
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		msg, err := c.reader.FetchMessage(ctx)
		if err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			c.log.Error("fetch message", "error", err)
			continue
		}

		var evt readingEvent
		if err := json.Unmarshal(msg.Value, &evt); err != nil {
			c.log.Warn("unmarshal reading event — skipping",
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

		push := &realtime.PushMessage{
			ChannelID: evt.Reading.ChannelID,
			DeviceID:  evt.Reading.DeviceID,
			Fields:    evt.Reading.Fields,
			Timestamp: evt.Reading.Timestamp,
			Type:      "reading",
		}
		c.hub.Broadcast(push)

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
func (c *ReadingConsumer) Close() error {
	return c.reader.Close()
}
