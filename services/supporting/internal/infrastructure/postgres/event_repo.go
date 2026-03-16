package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/greenlab/supporting/internal/application"
	"github.com/greenlab/supporting/internal/domain/audit"
)

// auditEventRow is the DB-layer representation of an audit event.
type auditEventRow struct {
	ID           uuid.UUID `db:"id"`
	TenantID     string    `db:"tenant_id"`
	UserID       string    `db:"user_id"`
	EventType    string    `db:"event_type"`
	ResourceID   string    `db:"resource_id"`
	ResourceType string    `db:"resource_type"`
	IPAddress    string    `db:"ip_address"`
	UserAgent    string    `db:"user_agent"`
	Payload      []byte    `db:"payload"`
	CreatedAt    time.Time `db:"created_at"`
}

func toAuditEvent(r *auditEventRow) *audit.AuditEvent {
	return &audit.AuditEvent{
		ID:           r.ID,
		TenantID:     r.TenantID,
		UserID:       r.UserID,
		EventType:    r.EventType,
		ResourceID:   r.ResourceID,
		ResourceType: r.ResourceType,
		IPAddress:    r.IPAddress,
		UserAgent:    r.UserAgent,
		Payload:      r.Payload,
		CreatedAt:    r.CreatedAt,
	}
}

// EventRepo is an append-only PostgreSQL store for audit events.
type EventRepo struct{ db *sqlx.DB }

func NewEventRepo(db *sqlx.DB) *EventRepo { return &EventRepo{db: db} }

// Append inserts a new audit event. This is the only write operation (no update/delete).
func (r *EventRepo) Append(ctx context.Context, e *audit.AuditEvent) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO audit_events (id, tenant_id, user_id, event_type, resource_id, resource_type, ip_address, user_agent, payload, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)`,
		e.ID, e.TenantID, e.UserID, e.EventType,
		e.ResourceID, e.ResourceType, e.IPAddress, e.UserAgent,
		e.Payload, e.CreatedAt,
	)
	return err
}

func (r *EventRepo) GetByID(ctx context.Context, id uuid.UUID) (*audit.AuditEvent, error) {
	var row auditEventRow
	err := r.db.GetContext(ctx, &row,
		`SELECT id, tenant_id, user_id, event_type, resource_id, resource_type, ip_address, user_agent, payload, created_at FROM audit_events WHERE id=$1`,
		id)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, audit.ErrEventNotFound
	}
	if err != nil {
		return nil, err
	}
	return toAuditEvent(&row), nil
}

func (r *EventRepo) ListByTenant(ctx context.Context, tenantID string, filter application.ListTenantFilter, limit, offset int) ([]*audit.AuditEvent, int64, error) {
	args := []interface{}{tenantID}
	conds := []string{"tenant_id=$1"}

	if filter.ResourceType != "" {
		args = append(args, filter.ResourceType)
		conds = append(conds, fmt.Sprintf("resource_type=$%d", len(args)))
	}
	if filter.Search != "" {
		args = append(args, "%"+strings.ToLower(filter.Search)+"%")
		idx := fmt.Sprintf("$%d", len(args))
		conds = append(conds, fmt.Sprintf("(LOWER(user_id) LIKE %s OR LOWER(event_type) LIKE %s OR LOWER(resource_id) LIKE %s)", idx, idx, idx))
	}

	where := strings.Join(conds, " AND ")
	args = append(args, limit, offset)
	limitIdx := len(args) - 1
	offsetIdx := len(args)

	query := fmt.Sprintf(
		`SELECT id, tenant_id, user_id, event_type, resource_id, resource_type, ip_address, user_agent, payload, created_at
		 FROM audit_events WHERE %s ORDER BY created_at DESC LIMIT $%d OFFSET $%d`,
		where, limitIdx, offsetIdx)

	var rows []*auditEventRow
	if err := r.db.SelectContext(ctx, &rows, query, args...); err != nil {
		return nil, 0, err
	}

	countArgs := args[:len(args)-2] // strip limit/offset
	countQuery := fmt.Sprintf(`SELECT COUNT(*) FROM audit_events WHERE %s`, where)
	var total int64
	if err := r.db.GetContext(ctx, &total, countQuery, countArgs...); err != nil {
		return nil, 0, err
	}

	events := make([]*audit.AuditEvent, len(rows))
	for i, row := range rows {
		events[i] = toAuditEvent(row)
	}
	return events, total, nil
}

func (r *EventRepo) ListByResource(ctx context.Context, resourceType, resourceID string, limit, offset int) ([]*audit.AuditEvent, int64, error) {
	var rows []*auditEventRow
	err := r.db.SelectContext(ctx, &rows,
		`SELECT id, tenant_id, user_id, event_type, resource_id, resource_type, ip_address, user_agent, payload, created_at FROM audit_events WHERE resource_type=$1 AND resource_id=$2 ORDER BY created_at DESC LIMIT $3 OFFSET $4`,
		resourceType, resourceID, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	var total int64
	if err := r.db.GetContext(ctx, &total,
		`SELECT COUNT(*) FROM audit_events WHERE resource_type=$1 AND resource_id=$2`, resourceType, resourceID); err != nil {
		return nil, 0, err
	}
	events := make([]*audit.AuditEvent, len(rows))
	for i, row := range rows {
		events[i] = toAuditEvent(row)
	}
	return events, total, nil
}
