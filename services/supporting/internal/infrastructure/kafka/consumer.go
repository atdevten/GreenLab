package kafka

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"time"

	"github.com/segmentio/kafka-go"
	"github.com/greenlab/supporting/internal/application"
	"github.com/greenlab/supporting/internal/domain/audit"
)

// AuditRecorder is the local interface the consumer depends on.
type AuditRecorder interface {
	Record(ctx context.Context, in application.RecordInput) (*audit.AuditEvent, error)
}

// AuditConsumer listens to event topics and records them as audit events.
type AuditConsumer struct {
	reader *kafka.Reader
	svc    AuditRecorder
	log    *slog.Logger
}

func NewAuditConsumer(brokers []string, groupID string, topics []string, svc AuditRecorder, log *slog.Logger) *AuditConsumer {
	var topic string
	if len(topics) > 0 {
		topic = topics[0]
	}
	reader := kafka.NewReader(kafka.ReaderConfig{
		Brokers:     brokers,
		Topic:       topic,
		GroupID:     groupID,
		MinBytes:    1,
		MaxBytes:    10e6,
		MaxWait:     500 * time.Millisecond,
		StartOffset: kafka.FirstOffset,
	})
	return &AuditConsumer{reader: reader, svc: svc, log: log}
}

type genericEvent struct {
	ID      string          `json:"id"`
	Type    string          `json:"type"`
	Source  string          `json:"source"`
	Payload json.RawMessage `json:"payload"`
}

type userPayload struct {
	UserID   string `json:"user_id"`
	TenantID string `json:"tenant_id"`
	Name     string `json:"name"`
	Email    string `json:"email"`
	IP       string `json:"ip"`
}

func (c *AuditConsumer) Start(ctx context.Context) error {
	c.log.Info("audit kafka consumer started")
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

		var evt genericEvent
		if err := json.Unmarshal(msg.Value, &evt); err != nil {
			c.log.Warn("unmarshal event envelope — skipping",
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

		var up userPayload
		tenantID, userID, userName, ipAddress := "", "", "", ""
		if err := json.Unmarshal(evt.Payload, &up); err == nil {
			tenantID = up.TenantID
			userID = up.UserID
			ipAddress = up.IP
			switch {
			case up.Name != "":
				userName = up.Name
			case up.Email != "":
				userName = up.Email
			default:
				userName = up.UserID
			}
		}

		if _, err := c.svc.Record(ctx, application.RecordInput{
			TenantID:     tenantID,
			UserID:       userID,
			UserName:     userName,
			EventType:    evt.Type,
			ResourceID:   string(msg.Key),
			ResourceType: sourceToResourceType(evt.Source),
			IPAddress:    ipAddress,
			UserAgent:    "kafka-consumer",
			Payload:      msg.Value,
		}); err != nil {
			c.log.Error("record audit event",
				"event_type", evt.Type,
				"offset", msg.Offset,
				"partition", msg.Partition,
				"error", err,
			)
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

func (c *AuditConsumer) Close() error {
	return c.reader.Close()
}

func sourceToResourceType(source string) string {
	if source == "" {
		return "unknown"
	}
	return source
}
