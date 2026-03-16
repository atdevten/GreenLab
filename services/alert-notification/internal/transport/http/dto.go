package http

import "time"

// Alert rule DTOs
type CreateRuleRequest struct {
	ChannelID   string   `json:"channel_id"   validate:"required"`
	WorkspaceID string   `json:"workspace_id" validate:"required"`
	Name        string   `json:"name"         validate:"required"`
	FieldName   string   `json:"field_name"   validate:"required"`
	Condition   string   `json:"condition"    validate:"required"`
	Threshold   *float64 `json:"threshold"    validate:"required"`
	Severity    string   `json:"severity"     validate:"required"`
	Message     string   `json:"message"`
	CooldownSec int      `json:"cooldown_sec"`
}

type UpdateRuleRequest struct {
	Name        string   `json:"name"`
	Threshold   *float64 `json:"threshold"`
	Severity    string   `json:"severity"`
	Message     string   `json:"message"`
	Enabled     *bool    `json:"enabled"`
	CooldownSec *int     `json:"cooldown_sec"`
}

type RuleResponse struct {
	ID          string    `json:"id"`
	ChannelID   string    `json:"channel_id"`
	WorkspaceID string    `json:"workspace_id"`
	Name        string    `json:"name"`
	FieldName   string    `json:"field_name"`
	Condition   string    `json:"condition"`
	Threshold   float64   `json:"threshold"`
	Severity    string    `json:"severity"`
	Message     string    `json:"message"`
	Enabled     bool      `json:"enabled"`
	CooldownSec int       `json:"cooldown_sec"`
	CreatedAt   time.Time `json:"created_at"`
}

// Notification DTOs
type SendNotificationRequest struct {
	WorkspaceID string `json:"workspace_id" validate:"required"`
	ChannelType string `json:"channel_type" validate:"required"`
	Recipient   string `json:"recipient"    validate:"required"`
	Subject     string `json:"subject"      validate:"required"`
	Body        string `json:"body"         validate:"required"`
}

type NotificationResponse struct {
	ID          string     `json:"id"`
	WorkspaceID string     `json:"workspace_id"`
	ChannelType string     `json:"channel_type"`
	Recipient   string     `json:"recipient"`
	Subject     string     `json:"subject"`
	Status      string     `json:"status"`
	Retries     int        `json:"retries"`
	SentAt      *time.Time `json:"sent_at"`
	Read        bool       `json:"read"`
	ReadAt      *time.Time `json:"read_at,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
}
