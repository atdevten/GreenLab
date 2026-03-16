package kafka

import (
	"context"
	"encoding/json"
	"time"

	"github.com/segmentio/kafka-go"
)

// ProducerConfig holds Kafka producer configuration.
type ProducerConfig struct {
	Brokers  []string
	Topic    string
	Async    bool
	BatchMax int
}

// Producer wraps a kafka-go writer.
type Producer struct {
	writer *kafka.Writer
}

// NewProducer creates a new Kafka producer.
func NewProducer(cfg ProducerConfig) *Producer {
	batchMax := cfg.BatchMax
	if batchMax <= 0 {
		batchMax = 100
	}

	w := &kafka.Writer{
		Addr:         kafka.TCP(cfg.Brokers...),
		Topic:        cfg.Topic,
		Balancer:     &kafka.LeastBytes{},
		BatchSize:    batchMax,
		BatchTimeout: 10 * time.Millisecond,
		Async:        cfg.Async,
		RequiredAcks: kafka.RequireOne,
		Compression:  kafka.Snappy,
	}

	return &Producer{writer: w}
}

// Publish sends a single message with the given key and JSON-encoded value.
func (p *Producer) Publish(ctx context.Context, key string, value interface{}) error {
	payload, err := json.Marshal(value)
	if err != nil {
		return err
	}

	msg := kafka.Message{
		Key:   []byte(key),
		Value: payload,
		Time:  time.Now(),
	}

	return p.writer.WriteMessages(ctx, msg)
}

// PublishRaw sends a raw byte message.
func (p *Producer) PublishRaw(ctx context.Context, key string, value []byte) error {
	msg := kafka.Message{
		Key:   []byte(key),
		Value: value,
		Time:  time.Now(),
	}
	return p.writer.WriteMessages(ctx, msg)
}

// PublishBatch sends multiple messages in a single batch.
func (p *Producer) PublishBatch(ctx context.Context, messages []kafka.Message) error {
	return p.writer.WriteMessages(ctx, messages...)
}

// Close closes the producer.
func (p *Producer) Close() error {
	return p.writer.Close()
}

// Event is a generic event envelope.
type Event struct {
	ID        string          `json:"id"`
	Type      string          `json:"type"`
	Source    string          `json:"source"`
	Timestamp time.Time       `json:"timestamp"`
	Payload   json.RawMessage `json:"payload"`
}

// NewEvent creates a new Event with JSON-encoded payload.
func NewEvent(id, eventType, source string, payload interface{}) (*Event, error) {
	raw, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	return &Event{
		ID:        id,
		Type:      eventType,
		Source:    source,
		Timestamp: time.Now(),
		Payload:   raw,
	}, nil
}
