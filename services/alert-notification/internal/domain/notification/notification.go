package notification

import (
	"time"

	"github.com/google/uuid"
)

// NotificationStatus tracks delivery state.
type NotificationStatus string

const (
	NotificationStatusPending NotificationStatus = "pending"
	NotificationStatusSent    NotificationStatus = "sent"
	NotificationStatusFailed  NotificationStatus = "failed"
)

// NotificationChannelType is the delivery channel type.
type NotificationChannelType string

const (
	ChannelTypeEmail   NotificationChannelType = "email"
	ChannelTypeWebhook NotificationChannelType = "webhook"
	ChannelTypeSMS     NotificationChannelType = "sms"
)

// Notification is a queued notification to be delivered.
type Notification struct {
	ID          uuid.UUID
	WorkspaceID uuid.UUID
	ChannelType NotificationChannelType
	Recipient   string
	Subject     string
	Body        string
	Status      NotificationStatus
	Retries     int
	SentAt      *time.Time
	ErrorMsg    string
	Read        bool
	ReadAt      *time.Time
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// NewNotification creates a new Notification entity.
func NewNotification(workspaceID uuid.UUID, channelType NotificationChannelType, recipient, subject, body string) *Notification {
	now := time.Now().UTC()
	return &Notification{
		ID:          uuid.New(),
		WorkspaceID: workspaceID,
		ChannelType: channelType,
		Recipient:   recipient,
		Subject:     subject,
		Body:        body,
		Status:      NotificationStatusPending,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
}

// MarkSent marks the notification as successfully delivered.
func (n *Notification) MarkSent() {
	now := time.Now().UTC()
	n.Status = NotificationStatusSent
	n.SentAt = &now
	n.UpdatedAt = now
}

// MarkFailed marks the notification as failed with an error message.
func (n *Notification) MarkFailed(errMsg string) {
	n.Status = NotificationStatusFailed
	n.ErrorMsg = errMsg
	n.Retries++
	n.UpdatedAt = time.Now().UTC()
}

// MarkRead marks the notification as read.
func (n *Notification) MarkRead() {
	now := time.Now().UTC()
	n.Read = true
	n.ReadAt = &now
	n.UpdatedAt = now
}

// Channel defines a notification delivery channel configured by a workspace.
type Channel struct {
	ID          uuid.UUID
	WorkspaceID uuid.UUID
	Name        string
	Type        NotificationChannelType
	Config      []byte // JSONB: channel-specific settings
	Enabled     bool
	CreatedAt   time.Time
}

// EmailConfig holds SMTP target settings for an email channel.
type EmailConfig struct {
	ToAddresses []string `json:"to_addresses"`
}

// WebhookConfig holds HTTP settings for a webhook channel.
type WebhookConfig struct {
	URL     string            `json:"url"`
	Method  string            `json:"method"`
	Headers map[string]string `json:"headers"`
}
