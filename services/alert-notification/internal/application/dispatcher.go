package application

import (
	"context"
	"fmt"

	"github.com/greenlab/alert-notification/internal/domain/notification"
)

// EmailSender defines the contract for email delivery.
type EmailSender interface {
	Send(ctx context.Context, to, subject, body string) error
}

// WebhookClient defines the contract for webhook delivery.
type WebhookClient interface {
	Post(ctx context.Context, url, payload string) error
}

// Dispatcher routes notifications to the correct delivery channel.
type Dispatcher struct {
	emailSender   EmailSender
	webhookClient WebhookClient
}

// NewDispatcher creates a new Dispatcher.
func NewDispatcher(emailSender EmailSender, webhookClient WebhookClient) *Dispatcher {
	return &Dispatcher{emailSender: emailSender, webhookClient: webhookClient}
}

// Dispatch sends a notification via its configured channel.
func (d *Dispatcher) Dispatch(ctx context.Context, n *notification.Notification) error {
	switch n.ChannelType {
	case notification.ChannelTypeEmail:
		return d.emailSender.Send(ctx, n.Recipient, n.Subject, n.Body)
	case notification.ChannelTypeWebhook:
		return d.webhookClient.Post(ctx, n.Recipient, n.Body)
	default:
		return fmt.Errorf("unsupported channel type: %s", n.ChannelType)
	}
}
