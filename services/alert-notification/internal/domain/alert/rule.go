package alert

import (
	"time"

	"github.com/google/uuid"
)

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
	ID          uuid.UUID
	ChannelID   uuid.UUID
	WorkspaceID uuid.UUID
	Name        string
	FieldName   string
	Condition   RuleCondition
	Threshold   float64
	Severity    RuleSeverity
	Message     string
	Enabled     bool
	CooldownSec int
	CreatedAt   time.Time
	UpdatedAt   time.Time
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
	ID          uuid.UUID     `json:"id"`
	RuleID      uuid.UUID     `json:"rule_id"`
	ChannelID   uuid.UUID     `json:"channel_id"`
	WorkspaceID uuid.UUID     `json:"workspace_id"`
	FieldName   string        `json:"field_name"`
	ActualValue float64       `json:"actual_value"`
	Threshold   float64       `json:"threshold"`
	Condition   RuleCondition `json:"condition"`
	Severity    RuleSeverity  `json:"severity"`
	Message     string        `json:"message"`
	TriggeredAt time.Time     `json:"triggered_at"`
}
