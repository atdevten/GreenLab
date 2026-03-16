package kafka

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/segmentio/kafka-go"
	"github.com/greenlab/alert-notification/internal/domain/alert"
)

const topicAlerts = "alert.events"

// AlertProducer publishes alert events to Kafka.
type AlertProducer struct {
	writer *kafka.Writer
}

// NewAlertProducer creates a new AlertProducer.
func NewAlertProducer(brokers []string) *AlertProducer {
	return &AlertProducer{
		writer: &kafka.Writer{
			Addr:         kafka.TCP(brokers...),
			Topic:        topicAlerts,
			Balancer:     &kafka.LeastBytes{},
			RequiredAcks: kafka.RequireAll,
		},
	}
}

// Close closes the underlying Kafka writer.
func (p *AlertProducer) Close() error { return p.writer.Close() }

type alertEnvelope struct {
	ID          string            `json:"id"`
	Type        string            `json:"type"`
	PublishedAt time.Time         `json:"published_at"`
	Event       *alert.AlertEvent `json:"event"`
}

// PublishAlert serialises and sends an alert event to the alert.events topic.
func (p *AlertProducer) PublishAlert(ctx context.Context, event *alert.AlertEvent) error {
	env := alertEnvelope{
		ID:          uuid.New().String(),
		Type:        "alert.triggered",
		PublishedAt: time.Now().UTC(),
		Event:       event,
	}
	b, err := json.Marshal(env)
	if err != nil {
		return err
	}
	return p.writer.WriteMessages(ctx, kafka.Message{
		Key:   []byte(event.ChannelID.String()),
		Value: b,
	})
}
