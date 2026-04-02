package application

import (
	"context"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/greenlab/alert-notification/internal/domain/alert"
	"github.com/greenlab/alert-notification/internal/domain/delivery"
)

// AlertPublisher publishes alert events to an event bus.
type AlertPublisher interface {
	PublishAlert(ctx context.Context, event *alert.AlertEvent) error
}

// AlertService manages alert rules.
type AlertService struct {
	repo         alert.RuleRepository
	publisher    AlertPublisher
	deliveryRepo delivery.Repository
	logger       *slog.Logger
}

// NewAlertService creates a new AlertService.
func NewAlertService(repo alert.RuleRepository, publisher AlertPublisher, deliveryRepo delivery.Repository, logger *slog.Logger) *AlertService {
	return &AlertService{repo: repo, publisher: publisher, deliveryRepo: deliveryRepo, logger: logger}
}

// CreateRuleInput holds the fields needed to create a new alert rule.
type CreateRuleInput struct {
	ChannelID     string
	WorkspaceID   string
	Name          string
	FieldName     string
	Condition     string
	Threshold     float64
	Severity      string
	Message       string
	CooldownSec   int
	WebhookSecret string // optional raw HMAC signing key
}

// CreateRule validates the input and persists a new alert rule.
func (s *AlertService) CreateRule(ctx context.Context, in CreateRuleInput) (*alert.Rule, error) {
	chID, err := uuid.Parse(in.ChannelID)
	if err != nil {
		return nil, alert.ErrInvalidChannelID
	}
	wsID, err := uuid.Parse(in.WorkspaceID)
	if err != nil {
		return nil, alert.ErrInvalidWorkspace
	}
	rule := alert.NewRule(chID, wsID, in.Name, in.FieldName,
		alert.RuleCondition(in.Condition), in.Threshold, alert.RuleSeverity(in.Severity))
	rule.Message = in.Message
	if in.CooldownSec > 0 {
		rule.CooldownSec = in.CooldownSec
	}
	rule.WebhookSecret = in.WebhookSecret
	if err := s.repo.Create(ctx, rule); err != nil {
		return nil, err
	}
	return rule, nil
}

// GetRule fetches a rule by its ID string.
func (s *AlertService) GetRule(ctx context.Context, id string) (*alert.Rule, error) {
	uid, err := uuid.Parse(id)
	if err != nil {
		return nil, alert.ErrInvalidRuleID
	}
	return s.repo.GetByID(ctx, uid)
}

// ListRules returns paginated rules for a workspace.
func (s *AlertService) ListRules(ctx context.Context, workspaceID string, limit, offset int) ([]*alert.Rule, int64, error) {
	wsID, err := uuid.Parse(workspaceID)
	if err != nil {
		return nil, 0, alert.ErrInvalidWorkspace
	}
	return s.repo.ListByWorkspace(ctx, wsID, limit, offset)
}

// UpdateRuleInput holds the optional fields for a partial rule update.
type UpdateRuleInput struct {
	Name          string
	Threshold     *float64
	Severity      string
	Message       string
	Enabled       *bool
	CooldownSec   *int
	WebhookSecret *string // nil = leave unchanged; pointer to empty string = clear the secret
}

// UpdateRule applies a partial update to an existing rule.
func (s *AlertService) UpdateRule(ctx context.Context, id string, in UpdateRuleInput) (*alert.Rule, error) {
	uid, err := uuid.Parse(id)
	if err != nil {
		return nil, alert.ErrInvalidRuleID
	}
	rule, err := s.repo.GetByID(ctx, uid)
	if err != nil {
		return nil, err
	}
	if in.Name != "" {
		rule.Name = in.Name
	}
	if in.Threshold != nil {
		rule.Threshold = *in.Threshold
	}
	if in.Severity != "" {
		rule.Severity = alert.RuleSeverity(in.Severity)
	}
	if in.Message != "" {
		rule.Message = in.Message
	}
	if in.Enabled != nil {
		rule.Enabled = *in.Enabled
	}
	if in.CooldownSec != nil {
		rule.CooldownSec = *in.CooldownSec
	}
	if in.WebhookSecret != nil {
		rule.WebhookSecret = *in.WebhookSecret
	}
	rule.UpdatedAt = time.Now().UTC()
	if err := s.repo.Update(ctx, rule); err != nil {
		return nil, err
	}
	return rule, nil
}

// DeleteRule removes a rule by its ID string.
func (s *AlertService) DeleteRule(ctx context.Context, id string) error {
	uid, err := uuid.Parse(id)
	if err != nil {
		return alert.ErrInvalidRuleID
	}
	return s.repo.Delete(ctx, uid)
}

// ListDeliveries returns paginated webhook delivery logs for a rule.
func (s *AlertService) ListDeliveries(ctx context.Context, ruleID string, limit, offset int) ([]*delivery.Log, int64, error) {
	uid, err := uuid.Parse(ruleID)
	if err != nil {
		return nil, 0, alert.ErrInvalidRuleID
	}
	return s.deliveryRepo.ListByRule(ctx, uid, limit, offset)
}

// VerifyWebhookSignature fetches the rule, then reports whether the provided
// signature matches HMAC-SHA256(rule.WebhookSecret, payload).
// Returns ErrNoWebhookSecret when the rule has no secret configured.
func (s *AlertService) VerifyWebhookSignature(ctx context.Context, id, payload, signature string) (bool, error) {
	uid, err := uuid.Parse(id)
	if err != nil {
		return false, alert.ErrInvalidRuleID
	}
	rule, err := s.repo.GetByID(ctx, uid)
	if err != nil {
		return false, err
	}
	if rule.WebhookSecret == "" {
		return false, alert.ErrNoWebhookSecret
	}
	return alert.ValidateHMAC(rule.WebhookSecret, payload, signature), nil
}
