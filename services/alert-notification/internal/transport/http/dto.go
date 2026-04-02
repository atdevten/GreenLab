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
	Secret      string   `json:"secret"` // optional raw HMAC signing key for webhook payloads
}

type UpdateRuleRequest struct {
	Name        string   `json:"name"`
	Threshold   *float64 `json:"threshold"`
	Severity    string   `json:"severity"`
	Message     string   `json:"message"`
	Enabled     *bool    `json:"enabled"`
	CooldownSec *int     `json:"cooldown_sec"`
	Secret      *string  `json:"secret"` // nil = leave unchanged; pointer to "" = clear the secret
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

// DeliveryLogResponse is the API representation of a webhook delivery log entry.
type DeliveryLogResponse struct {
	ID           string    `json:"id"`
	RuleID       string    `json:"rule_id"`
	URL          string    `json:"url"`
	HTTPStatus   int       `json:"http_status"`
	LatencyMS    int64     `json:"latency_ms"`
	ResponseBody string    `json:"response_body"`
	ErrorMsg     string    `json:"error_msg"`
	DeliveredAt  time.Time `json:"delivered_at"`
}

// VerifySignatureRequest is the request body for the sandbox verify-signature endpoint.
type VerifySignatureRequest struct {
	Payload   string `json:"payload"   validate:"required"`
	Signature string `json:"signature" validate:"required"`
}

// VerifySignatureResponse reports whether the HMAC-SHA256 signature is valid.
type VerifySignatureResponse struct {
	Valid bool `json:"valid"`
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
