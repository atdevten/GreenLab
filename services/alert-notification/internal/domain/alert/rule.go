package alert

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"time"

	"github.com/google/uuid"
)

// HashSecret returns the hex-encoded SHA-256 hash of secret.
// It is used only as a one-way fingerprint to detect whether a secret
// has been set; the raw secret is what is actually stored for HMAC signing.
// This helper is intentionally kept simple so callers can use it for
// comparison without needing to import crypto packages directly.
func HashSecret(secret string) string {
	sum := sha256.Sum256([]byte(secret))
	return hex.EncodeToString(sum[:])
}

// ComputeHMAC returns the hex-encoded HMAC-SHA256 of payload keyed by secret.
func ComputeHMAC(secret, payload string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(payload))
	return hex.EncodeToString(mac.Sum(nil))
}

// ValidateHMAC reports whether the provided signature matches the expected
// HMAC-SHA256 of payload under secret. Uses hmac.Equal for constant-time
// comparison to prevent timing attacks.
func ValidateHMAC(secret, payload, signature string) bool {
	expected := "sha256=" + ComputeHMAC(secret, payload)
	return hmac.Equal([]byte(expected), []byte(signature))
}

// RuleCondition defines how to evaluate a field value.
type RuleCondition string

const (
	ConditionGT  RuleCondition = "gt"
	ConditionGTE RuleCondition = "gte"
	ConditionLT  RuleCondition = "lt"
	ConditionLTE RuleCondition = "lte"
	ConditionEQ  RuleCondition = "eq"
	ConditionNEQ RuleCondition = "neq"
)

// RuleSeverity defines the alert severity level.
type RuleSeverity string

const (
	SeverityInfo     RuleSeverity = "info"
	SeverityWarning  RuleSeverity = "warning"
	SeverityCritical RuleSeverity = "critical"
)

// Rule defines an alerting rule evaluated against telemetry data.
type Rule struct {
	ID            uuid.UUID
	ChannelID     uuid.UUID
	WorkspaceID   uuid.UUID
	Name          string
	FieldName     string
	Condition     RuleCondition
	Threshold     float64
	Severity      RuleSeverity
	Message       string
	Enabled       bool
	CooldownSec   int
	WebhookSecret string // raw HMAC signing key; stored as-is in webhook_secret_hash column
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

// NewRule creates a new alert rule.
func NewRule(channelID, workspaceID uuid.UUID, name, fieldName string, condition RuleCondition, threshold float64, severity RuleSeverity) *Rule {
	now := time.Now().UTC()
	return &Rule{
		ID:          uuid.New(),
		ChannelID:   channelID,
		WorkspaceID: workspaceID,
		Name:        name,
		FieldName:   fieldName,
		Condition:   condition,
		Threshold:   threshold,
		Severity:    severity,
		Enabled:     true,
		CooldownSec: 300,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
}

// Evaluate returns true if the given value triggers this rule.
func (r *Rule) Evaluate(value float64) bool {
	switch r.Condition {
	case ConditionGT:
		return value > r.Threshold
	case ConditionGTE:
		return value >= r.Threshold
	case ConditionLT:
		return value < r.Threshold
	case ConditionLTE:
		return value <= r.Threshold
	case ConditionEQ:
		return value == r.Threshold
	case ConditionNEQ:
		return value != r.Threshold
	}
	return false
}

// AlertEvent represents a triggered alert.
type AlertEvent struct {
	ID            uuid.UUID     `json:"id"`
	RuleID        uuid.UUID     `json:"rule_id"`
	ChannelID     uuid.UUID     `json:"channel_id"`
	WorkspaceID   uuid.UUID     `json:"workspace_id"`
	FieldName     string        `json:"field_name"`
	ActualValue   float64       `json:"actual_value"`
	Threshold     float64       `json:"threshold"`
	Condition     RuleCondition `json:"condition"`
	Severity      RuleSeverity  `json:"severity"`
	Message       string        `json:"message"`
	TriggeredAt   time.Time     `json:"triggered_at"`
	WebhookSecret string        `json:"webhook_secret,omitempty"` // raw HMAC key; omitted from JSON when empty
}
