package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/greenlab/alert-notification/internal/domain/delivery"
)

type deliveryRow struct {
	ID           uuid.UUID `db:"id"`
	RuleID       uuid.UUID `db:"rule_id"`
	URL          string    `db:"url"`
	HTTPStatus   int       `db:"http_status"`
	LatencyMS    int64     `db:"latency_ms"`
	ResponseBody string    `db:"response_body"`
	ErrorMsg     string    `db:"error_msg"`
	DeliveredAt  time.Time `db:"delivered_at"`
}

func toDeliveryLog(r *deliveryRow) *delivery.Log {
	return &delivery.Log{
		ID:           r.ID,
		RuleID:       r.RuleID,
		URL:          r.URL,
		HTTPStatus:   r.HTTPStatus,
		LatencyMS:    r.LatencyMS,
		ResponseBody: r.ResponseBody,
		ErrorMsg:     r.ErrorMsg,
		DeliveredAt:  r.DeliveredAt,
	}
}

// DeliveryRepo implements delivery.Repository using PostgreSQL.
type DeliveryRepo struct{ db *sqlx.DB }

// NewDeliveryRepo creates a new DeliveryRepo.
func NewDeliveryRepo(db *sqlx.DB) *DeliveryRepo { return &DeliveryRepo{db: db} }

// Save inserts a new webhook delivery log entry.
func (r *DeliveryRepo) Save(ctx context.Context, l *delivery.Log) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO webhook_delivery_logs (id, rule_id, url, http_status, latency_ms, response_body, error_msg, delivered_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
		l.ID, l.RuleID, l.URL, l.HTTPStatus, l.LatencyMS, l.ResponseBody, l.ErrorMsg, l.DeliveredAt)
	if err != nil {
		return fmt.Errorf("DeliveryRepo.Save: %w", err)
	}
	return nil
}

// ListByRule returns paginated delivery logs for a rule, ordered by delivered_at DESC.
func (r *DeliveryRepo) ListByRule(ctx context.Context, ruleID uuid.UUID, limit, offset int) ([]*delivery.Log, int64, error) {
	var rows []*deliveryRow
	err := r.db.SelectContext(ctx, &rows,
		`SELECT id, rule_id, url, http_status, latency_ms, response_body, error_msg, delivered_at
		 FROM webhook_delivery_logs
		 WHERE rule_id=$1
		 ORDER BY delivered_at DESC
		 LIMIT $2 OFFSET $3`,
		ruleID, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("DeliveryRepo.ListByRule: %w", err)
	}
	var total int64
	if err := r.db.GetContext(ctx, &total,
		`SELECT COUNT(*) FROM webhook_delivery_logs WHERE rule_id=$1`, ruleID); err != nil {
		return nil, 0, fmt.Errorf("DeliveryRepo.ListByRule count: %w", err)
	}
	logs := make([]*delivery.Log, len(rows))
	for i, row := range rows {
		logs[i] = toDeliveryLog(row)
	}
	return logs, total, nil
}
