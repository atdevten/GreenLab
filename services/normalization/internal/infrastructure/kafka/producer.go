package kafka

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/segmentio/kafka-go"
	"github.com/greenlab/normalization/internal/domain"
)

const topicNormalized = "normalized.sensor"

// NormalizedProducer publishes normalized reading events to Kafka.
type NormalizedProducer struct {
	writer *kafka.Writer
}

// NewNormalizedProducer creates a producer that writes to normalized.sensor.
func NewNormalizedProducer(brokers []string) *NormalizedProducer {
	return &NormalizedProducer{
		writer: &kafka.Writer{
			Addr:         kafka.TCP(brokers...),
			Topic:        topicNormalized,
			Balancer:     &kafka.Hash{},
			RequiredAcks: kafka.RequireOne,
			BatchSize:    500,
			BatchTimeout: 5 * time.Millisecond,
			Compression:  kafka.Snappy,
		},
	}
}

// PublishReading publishes a normalized reading event. The message key is the channel_id
// to ensure all readings for the same channel land on the same partition.
func (p *NormalizedProducer) PublishReading(ctx context.Context, evt *domain.ReadingEvent) error {
	b, err := json.Marshal(evt)
	if err != nil {
		return fmt.Errorf("NormalizedProducer.PublishReading: marshal: %w", err)
	}
	if err := p.writer.WriteMessages(ctx, kafka.Message{
		Key:   []byte(evt.Reading.ChannelID),
		Value: b,
	}); err != nil {
		return fmt.Errorf("NormalizedProducer.PublishReading: %w", err)
	}
	return nil
}

// Close closes the underlying Kafka writer.
func (p *NormalizedProducer) Close() error {
	return p.writer.Close()
}
