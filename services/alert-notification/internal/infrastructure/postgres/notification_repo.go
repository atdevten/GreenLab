package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/greenlab/alert-notification/internal/domain/notification"
)

// notificationRow is the DB-layer representation of a notification.
type notificationRow struct {
	ID          uuid.UUID                            `db:"id"`
	WorkspaceID uuid.UUID                            `db:"workspace_id"`
	ChannelType notification.NotificationChannelType `db:"channel_type"`
	Recipient   string                               `db:"recipient"`
	Subject     string                               `db:"subject"`
	Body        string                               `db:"body"`
	Status      notification.NotificationStatus      `db:"status"`
	Retries     int                                  `db:"retries"`
	SentAt      *time.Time                           `db:"sent_at"`
	ErrorMsg    string                               `db:"error_msg"`
	Read        bool                                 `db:"read"`
	ReadAt      *time.Time                           `db:"read_at"`
	CreatedAt   time.Time                            `db:"created_at"`
	UpdatedAt   time.Time                            `db:"updated_at"`
}

func toNotification(r *notificationRow) *notification.Notification {
	return &notification.Notification{
		ID:          r.ID,
		WorkspaceID: r.WorkspaceID,
		ChannelType: r.ChannelType,
		Recipient:   r.Recipient,
		Subject:     r.Subject,
		Body:        r.Body,
		Status:      r.Status,
		Retries:     r.Retries,
		SentAt:      r.SentAt,
		ErrorMsg:    r.ErrorMsg,
		Read:        r.Read,
		ReadAt:      r.ReadAt,
		CreatedAt:   r.CreatedAt,
		UpdatedAt:   r.UpdatedAt,
	}
}

func fromNotification(n *notification.Notification) *notificationRow {
	return &notificationRow{
		ID:          n.ID,
		WorkspaceID: n.WorkspaceID,
		ChannelType: n.ChannelType,
		Recipient:   n.Recipient,
		Subject:     n.Subject,
		Body:        n.Body,
		Status:      n.Status,
		Retries:     n.Retries,
		SentAt:      n.SentAt,
		ErrorMsg:    n.ErrorMsg,
		Read:        n.Read,
		ReadAt:      n.ReadAt,
		CreatedAt:   n.CreatedAt,
		UpdatedAt:   n.UpdatedAt,
	}
}

type NotificationRepo struct{ db *sqlx.DB }

func NewNotificationRepo(db *sqlx.DB) *NotificationRepo { return &NotificationRepo{db: db} }

func (r *NotificationRepo) Save(ctx context.Context, n *notification.Notification) error {
	_, err := r.db.NamedExecContext(ctx, `
		INSERT INTO notifications (id, workspace_id, channel_type, recipient, subject, body, status, retries, error_msg)
		VALUES (:id, :workspace_id, :channel_type, :recipient, :subject, :body, :status, :retries, :error_msg)`,
		fromNotification(n))
	return err
}

func (r *NotificationRepo) Update(ctx context.Context, n *notification.Notification) error {
	_, err := r.db.ExecContext(ctx, `
		UPDATE notifications SET status=$1, retries=$2, sent_at=$3, error_msg=$4, updated_at=NOW() WHERE id=$5`,
		n.Status, n.Retries, n.SentAt, n.ErrorMsg, n.ID)
	return err
}

func (r *NotificationRepo) GetByID(ctx context.Context, id uuid.UUID) (*notification.Notification, error) {
	var row notificationRow
	err := r.db.GetContext(ctx, &row, `SELECT * FROM notifications WHERE id=$1`, id)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, notification.ErrNotificationNotFound
	}
	if err != nil {
		return nil, err
	}
	return toNotification(&row), nil
}

func (r *NotificationRepo) MarkRead(ctx context.Context, id, tenantID string) error {
	res, err := r.db.ExecContext(ctx, `
		UPDATE notifications SET read=true, read_at=NOW(), updated_at=NOW()
		WHERE id=$1 AND workspace_id=$2`,
		id, tenantID)
	if err != nil {
		return fmt.Errorf("NotificationRepo.MarkRead: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return notification.ErrNotificationNotFound
	}
	return nil
}

func (r *NotificationRepo) MarkAllRead(ctx context.Context, tenantID string) error {
	_, err := r.db.ExecContext(ctx, `
		UPDATE notifications SET read=true, read_at=NOW(), updated_at=NOW()
		WHERE workspace_id=$1 AND read=false`, tenantID)
	if err != nil {
		return fmt.Errorf("NotificationRepo.MarkAllRead: %w", err)
	}
	return nil
}

func (r *NotificationRepo) ListByWorkspace(ctx context.Context, workspaceID uuid.UUID, limit, offset int) ([]*notification.Notification, int64, error) {
	var rows []*notificationRow
	err := r.db.SelectContext(ctx, &rows,
		`SELECT * FROM notifications WHERE workspace_id=$1 ORDER BY created_at DESC LIMIT $2 OFFSET $3`,
		workspaceID, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	var total int64
	if err := r.db.GetContext(ctx, &total, `SELECT COUNT(*) FROM notifications WHERE workspace_id=$1`, workspaceID); err != nil {
		return nil, 0, err
	}
	notifications := make([]*notification.Notification, len(rows))
	for i, row := range rows {
		notifications[i] = toNotification(row)
	}
	return notifications, total, nil
}
