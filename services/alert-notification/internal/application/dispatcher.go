package application

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/greenlab/alert-notification/internal/domain/alert"
	"github.com/greenlab/alert-notification/internal/domain/delivery"
	"github.com/greenlab/alert-notification/internal/domain/notification"
)

// EmailSender defines the contract for email delivery.
type EmailSender interface {
	Send(ctx context.Context, to, subject, body string) error
}

// WebhookClient defines the contract for webhook delivery.
type WebhookClient interface {
	Post(ctx context.Context, url, payload string) error
	// PostDetailed sends payload and optionally attaches an X-GreenLab-Signature header when
	// hmacSignature is non-empty (caller is responsible for formatting, e.g. "sha256=<hex>").
	PostDetailed(ctx context.Context, url, payload, hmacSignature string) (httpStatus int, responseBody string, latencyMS int64, err error)
}

// Dispatcher routes notifications to the correct delivery channel.
type Dispatcher struct {
	emailSender   EmailSender
	webhookClient WebhookClient
	deliveryRepo  delivery.Repository
	logger        *slog.Logger
}

// NewDispatcher creates a new Dispatcher.
func NewDispatcher(emailSender EmailSender, webhookClient WebhookClient, deliveryRepo delivery.Repository, logger *slog.Logger) *Dispatcher {
	return &Dispatcher{
		emailSender:   emailSender,
		webhookClient: webhookClient,
		deliveryRepo:  deliveryRepo,
		logger:        logger,
	}
}

// Dispatch sends a notification via its configured channel.
// For webhook channels, delivery details are persisted to the delivery log when
// the notification carries a RuleID.
func (d *Dispatcher) Dispatch(ctx context.Context, n *notification.Notification) error {
	switch n.ChannelType {
	case notification.ChannelTypeEmail:
		return d.emailSender.Send(ctx, n.Recipient, n.Subject, n.Body)
	case notification.ChannelTypeWebhook:
		return d.dispatchWebhook(ctx, n)
	default:
		return fmt.Errorf("unsupported channel type: %s", n.ChannelType)
	}
}

func (d *Dispatcher) dispatchWebhook(ctx context.Context, n *notification.Notification) error {
	var sig string
	if n.WebhookSecret != "" {
		sig = "sha256=" + alert.ComputeHMAC(n.WebhookSecret, n.Body)
	}

	httpStatus, respBody, latencyMS, err := d.webhookClient.PostDetailed(ctx, n.Recipient, n.Body, sig)

	if n.RuleID != nil {
		d.saveDeliveryLog(ctx, n.RuleID, n.Recipient, httpStatus, respBody, latencyMS, err)
	}

	if err != nil {
		return err
	}
	if httpStatus < 200 || httpStatus >= 300 {
		return fmt.Errorf("webhook returned non-2xx status: %d", httpStatus)
	}
	return nil
}

func (d *Dispatcher) saveDeliveryLog(ctx context.Context, ruleID *uuid.UUID, url string, httpStatus int, respBody string, latencyMS int64, dispatchErr error) {
	l := &delivery.Log{
		ID:           uuid.New(),
		RuleID:       *ruleID,
		URL:          url,
		HTTPStatus:   httpStatus,
		LatencyMS:    latencyMS,
		ResponseBody: respBody,
		DeliveredAt:  time.Now().UTC(),
	}
	if dispatchErr != nil {
		l.ErrorMsg = dispatchErr.Error()
	}
	if err := d.deliveryRepo.Save(ctx, l); err != nil {
		d.logger.Error("save delivery log failed", "rule_id", ruleID, "error", err)
	}
}
