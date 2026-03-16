package kafka

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"time"

	"github.com/segmentio/kafka-go"
	"github.com/greenlab/alert-notification/internal/domain/alert"
)

// AlertNotifier is the local interface for dispatching alert notifications.
type AlertNotifier interface {
	SendAlertNotification(ctx context.Context, evt *alert.AlertEvent) error
}

// AlertConsumer consumes alert events and triggers notifications.
type AlertConsumer struct {
	reader *kafka.Reader
	svc    AlertNotifier
	log    *slog.Logger
}

// NewAlertConsumer creates a new AlertConsumer.
func NewAlertConsumer(brokers []string, groupID string, svc AlertNotifier, log *slog.Logger) *AlertConsumer {
	reader := kafka.NewReader(kafka.ReaderConfig{
		Brokers:     brokers,
		Topic:       "alert.events",
		GroupID:     groupID,
		MinBytes:    1,
		MaxBytes:    10e6,
		MaxWait:     500 * time.Millisecond,
		StartOffset: kafka.FirstOffset,
	})
	return &AlertConsumer{reader: reader, svc: svc, log: log}
}

// alertEventEnvelope matches the envelope format written by AlertProducer.
type alertEventEnvelope struct {
	Type        string           `json:"type"`
	PublishedAt time.Time        `json:"published_at"`
	Event       alert.AlertEvent `json:"event"`
}

// Start begins consuming messages until ctx is cancelled.
func (c *AlertConsumer) Start(ctx context.Context) error {
	c.log.Info("notification alert kafka consumer started")
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

		var env alertEventEnvelope
		if err := json.Unmarshal(msg.Value, &env); err != nil {
			c.log.Warn("unmarshal alert envelope — skipping",
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

		if env.Type == "alert.triggered" {
			if err := c.svc.SendAlertNotification(ctx, &env.Event); err != nil {
				c.log.Error("send alert notification — skipping commit to allow retry",
					"rule_id", env.Event.RuleID,
					"channel_id", env.Event.ChannelID,
					"error", err,
				)
				continue
			}
		}

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
func (c *AlertConsumer) Close() error {
	return c.reader.Close()
}
