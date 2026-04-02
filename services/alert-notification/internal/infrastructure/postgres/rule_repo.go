package postgres

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/greenlab/alert-notification/internal/domain/alert"
)

// ruleRow is the DB-layer representation of an alert rule.
// It uses db: tags for sqlx scanning without polluting the domain struct.
type ruleRow struct {
	ID                uuid.UUID `db:"id"`
	ChannelID         uuid.UUID `db:"channel_id"`
	WorkspaceID       uuid.UUID `db:"workspace_id"`
	Name              string    `db:"name"`
	FieldName         string    `db:"field_name"`
	Condition         string    `db:"condition"`
	Threshold         float64   `db:"threshold"`
	Severity          string    `db:"severity"`
	Message           string    `db:"message"`
	Enabled           bool      `db:"enabled"`
	CooldownSec       int       `db:"cooldown_sec"`
	WebhookSecretHash string    `db:"webhook_secret_hash"`
	CreatedAt         time.Time `db:"created_at"`
	UpdatedAt         time.Time `db:"updated_at"`
}

func toRule(r *ruleRow) *alert.Rule {
	return &alert.Rule{
		ID:            r.ID,
		ChannelID:     r.ChannelID,
		WorkspaceID:   r.WorkspaceID,
		Name:          r.Name,
		FieldName:     r.FieldName,
		Condition:     alert.RuleCondition(r.Condition),
		Threshold:     r.Threshold,
		Severity:      alert.RuleSeverity(r.Severity),
		Message:       r.Message,
		Enabled:       r.Enabled,
		CooldownSec:   r.CooldownSec,
		WebhookSecret: r.WebhookSecretHash,
		CreatedAt:     r.CreatedAt,
		UpdatedAt:     r.UpdatedAt,
	}
}

func fromRule(r *alert.Rule) *ruleRow {
	return &ruleRow{
		ID:                r.ID,
		ChannelID:         r.ChannelID,
		WorkspaceID:       r.WorkspaceID,
		Name:              r.Name,
		FieldName:         r.FieldName,
		Condition:         string(r.Condition),
		Threshold:         r.Threshold,
		Severity:          string(r.Severity),
		Message:           r.Message,
		Enabled:           r.Enabled,
		CooldownSec:       r.CooldownSec,
		WebhookSecretHash: r.WebhookSecret,
		CreatedAt:         r.CreatedAt,
		UpdatedAt:         r.UpdatedAt,
	}
}

type RuleRepo struct{ db *sqlx.DB }

func NewRuleRepo(db *sqlx.DB) *RuleRepo { return &RuleRepo{db: db} }

func (r *RuleRepo) Create(ctx context.Context, rule *alert.Rule) error {
	_, err := r.db.NamedExecContext(ctx, `
		INSERT INTO alert_rules (id, channel_id, workspace_id, name, field_name, condition, threshold, severity, message, enabled, cooldown_sec, webhook_secret_hash)
		VALUES (:id, :channel_id, :workspace_id, :name, :field_name, :condition, :threshold, :severity, :message, :enabled, :cooldown_sec, :webhook_secret_hash)`,
		fromRule(rule))
	return err
}

func (r *RuleRepo) GetByID(ctx context.Context, id uuid.UUID) (*alert.Rule, error) {
	var row ruleRow
	err := r.db.GetContext(ctx, &row, `
		SELECT id, channel_id, workspace_id, name, field_name, condition, threshold, severity, message, enabled, cooldown_sec, webhook_secret_hash, created_at, updated_at
		FROM alert_rules WHERE id=$1`, id)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, alert.ErrRuleNotFound
	}
	if err != nil {
		return nil, err
	}
	return toRule(&row), nil
}

func (r *RuleRepo) ListByChannel(ctx context.Context, channelID uuid.UUID) ([]*alert.Rule, error) {
	var rows []*ruleRow
	err := r.db.SelectContext(ctx, &rows, `
		SELECT id, channel_id, workspace_id, name, field_name, condition, threshold, severity, message, enabled, cooldown_sec, webhook_secret_hash, created_at, updated_at
		FROM alert_rules WHERE channel_id=$1 ORDER BY created_at`, channelID)
	if err != nil {
		return nil, err
	}
	rules := make([]*alert.Rule, len(rows))
	for i, row := range rows {
		rules[i] = toRule(row)
	}
	return rules, nil
}

func (r *RuleRepo) ListByWorkspace(ctx context.Context, workspaceID uuid.UUID, limit, offset int) ([]*alert.Rule, int64, error) {
	var rows []*ruleRow
	err := r.db.SelectContext(ctx, &rows, `
		SELECT id, channel_id, workspace_id, name, field_name, condition, threshold, severity, message, enabled, cooldown_sec, webhook_secret_hash, created_at, updated_at
		FROM alert_rules WHERE workspace_id=$1 ORDER BY created_at DESC LIMIT $2 OFFSET $3`,
		workspaceID, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	var total int64
	if err := r.db.GetContext(ctx, &total, `SELECT COUNT(*) FROM alert_rules WHERE workspace_id=$1`, workspaceID); err != nil {
		return nil, 0, err
	}
	rules := make([]*alert.Rule, len(rows))
	for i, row := range rows {
		rules[i] = toRule(row)
	}
	return rules, total, nil
}

func (r *RuleRepo) Update(ctx context.Context, rule *alert.Rule) error {
	_, err := r.db.ExecContext(ctx, `
		UPDATE alert_rules SET name=$1, threshold=$2, severity=$3, message=$4, enabled=$5, cooldown_sec=$6, webhook_secret_hash=$7, updated_at=NOW() WHERE id=$8`,
		rule.Name, rule.Threshold, rule.Severity, rule.Message, rule.Enabled, rule.CooldownSec, rule.WebhookSecret, rule.ID)
	return err
}

func (r *RuleRepo) Delete(ctx context.Context, id uuid.UUID) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM alert_rules WHERE id=$1`, id)
	return err
}

func (r *RuleRepo) ListEnabled(ctx context.Context) ([]*alert.Rule, error) {
	var rows []*ruleRow
	err := r.db.SelectContext(ctx, &rows, `SELECT * FROM alert_rules WHERE enabled=TRUE`)
	if err != nil {
		return nil, err
	}
	rules := make([]*alert.Rule, len(rows))
	for i, row := range rows {
		rules[i] = toRule(row)
	}
	return rules, nil
}
