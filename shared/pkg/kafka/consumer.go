package kafka

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/segmentio/kafka-go"
)

// ConsumerConfig holds Kafka consumer configuration.
type ConsumerConfig struct {
	Brokers     []string
	Topic       string
	GroupID     string
	MinBytes    int
	MaxBytes    int
	MaxWait     time.Duration
	StartOffset int64
}

// Consumer wraps a kafka-go reader.
type Consumer struct {
	reader *kafka.Reader
	cfg    ConsumerConfig
}

// NewConsumer creates a new Kafka consumer.
func NewConsumer(cfg ConsumerConfig) *Consumer {
	minBytes := cfg.MinBytes
	if minBytes <= 0 {
		minBytes = 1
	}
	maxBytes := cfg.MaxBytes
	if maxBytes <= 0 {
		maxBytes = 10e6 // 10MB
	}
	maxWait := cfg.MaxWait
	if maxWait <= 0 {
		maxWait = 1 * time.Second
	}
	startOffset := cfg.StartOffset
	if startOffset == 0 {
		startOffset = kafka.LastOffset
	}

	r := kafka.NewReader(kafka.ReaderConfig{
		Brokers:     cfg.Brokers,
		Topic:       cfg.Topic,
		GroupID:     cfg.GroupID,
		MinBytes:    minBytes,
		MaxBytes:    int(maxBytes),
		MaxWait:     maxWait,
		StartOffset: startOffset,
	})

	return &Consumer{reader: r, cfg: cfg}
}

// MessageHandler is the function type for handling consumed messages.
type MessageHandler func(ctx context.Context, msg kafka.Message) error

// Consume starts consuming messages and calls the handler for each.
// It commits offsets only after successful handler execution.
func (c *Consumer) Consume(ctx context.Context, handler MessageHandler) error {
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
			return fmt.Errorf("fetch message: %w", err)
		}

		if err := handler(ctx, msg); err != nil {
			// Log error but continue (dead-letter queue pattern can be added here)
			continue
		}

		if err := c.reader.CommitMessages(ctx, msg); err != nil {
			return fmt.Errorf("commit message: %w", err)
		}
	}
}

// Close closes the consumer.
func (c *Consumer) Close() error {
	return c.reader.Close()
}

// UnmarshalPayload decodes a Kafka message's value into the target struct.
func UnmarshalPayload(msg kafka.Message, target interface{}) error {
	return json.Unmarshal(msg.Value, target)
}

// UnmarshalEvent decodes a Kafka message into an Event envelope.
func UnmarshalEvent(msg kafka.Message) (*Event, error) {
	var ev Event
	if err := json.Unmarshal(msg.Value, &ev); err != nil {
		return nil, err
	}
	return &ev, nil
}
