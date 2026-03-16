package kafka

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/segmentio/kafka-go"
	"github.com/greenlab/iam/internal/domain/auth"
)

const topicUserEvents = "user.events"

type EventProducer struct {
	writer *kafka.Writer
}

func NewEventProducer(brokers []string) *EventProducer {
	return &EventProducer{
		writer: &kafka.Writer{
			Addr:         kafka.TCP(brokers...),
			Topic:        topicUserEvents,
			Balancer:     &kafka.LeastBytes{},
			RequiredAcks: kafka.RequireAll,
		},
	}
}

func (p *EventProducer) Close() error { return p.writer.Close() }

type userEvent struct {
	ID        string          `json:"id"`
	Type      string          `json:"type"`
	Timestamp time.Time       `json:"timestamp"`
	Payload   json.RawMessage `json:"payload"`
}

func (p *EventProducer) publish(ctx context.Context, eventType, key string, payload interface{}) error {
	raw, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	evt := userEvent{ID: uuid.New().String(), Type: eventType, Timestamp: time.Now().UTC(), Payload: raw}
	b, err := json.Marshal(evt)
	if err != nil {
		return err
	}
	return p.writer.WriteMessages(ctx, kafka.Message{Key: []byte(key), Value: b})
}

func (p *EventProducer) PublishUserRegistered(ctx context.Context, user *auth.User) error {
	return p.publish(ctx, "user.registered", user.ID.String(), map[string]interface{}{
		"user_id": user.ID, "tenant_id": user.TenantID, "email": user.Email,
		"first_name": user.FirstName, "last_name": user.LastName,
	})
}

func (p *EventProducer) PublishUserLoggedIn(ctx context.Context, user *auth.User, ip string) error {
	return p.publish(ctx, "user.logged_in", user.ID.String(), map[string]interface{}{
		"user_id": user.ID, "email": user.Email, "ip": ip,
	})
}

func (p *EventProducer) PublishPasswordChanged(ctx context.Context, userID uuid.UUID) error {
	return p.publish(ctx, "user.password_changed", userID.String(), map[string]interface{}{"user_id": userID})
}

func (p *EventProducer) PublishEmailVerified(ctx context.Context, userID uuid.UUID) error {
	return p.publish(ctx, "user.email_verified", userID.String(), map[string]interface{}{"user_id": userID})
}
