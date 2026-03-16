package kafka

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/segmentio/kafka-go"
	"github.com/greenlab/normalization/internal/domain"
)

const topicRawIngest = "raw.sensor.ingest"

// ReadingConsumer reads raw reading events from Kafka.
type ReadingConsumer struct {
	reader *kafka.Reader
}

// NewReadingConsumer creates a consumer for the raw.sensor.ingest topic.
func NewReadingConsumer(brokers []string) *ReadingConsumer {
	return &ReadingConsumer{
		reader: kafka.NewReader(kafka.ReaderConfig{
			Brokers: brokers,
			Topic:   topicRawIngest,
			GroupID: "normalization-service",
		}),
	}
}

// ReadMessage blocks until a message is available, then deserializes it into a ReadingEvent.
func (c *ReadingConsumer) ReadMessage(ctx context.Context) (*domain.ReadingEvent, error) {
	msg, err := c.reader.ReadMessage(ctx)
	if err != nil {
		return nil, fmt.Errorf("ReadingConsumer.ReadMessage: %w", err)
	}
	var evt domain.ReadingEvent
	if err := json.Unmarshal(msg.Value, &evt); err != nil {
		return nil, fmt.Errorf("ReadingConsumer.ReadMessage: unmarshal: %w", err)
	}
	return &evt, nil
}

// Close closes the underlying Kafka reader.
func (c *ReadingConsumer) Close() error {
	return c.reader.Close()
}
