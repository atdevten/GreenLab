package application

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/google/uuid"
	"github.com/greenlab/alert-notification/internal/domain/alert"
	"github.com/greenlab/alert-notification/internal/domain/notification"
)

// NotificationRepository persists notifications.
type NotificationRepository interface {
	Save(ctx context.Context, n *notification.Notification) error
	Update(ctx context.Context, n *notification.Notification) error
	ListByWorkspace(ctx context.Context, workspaceID uuid.UUID, limit, offset int) ([]*notification.Notification, int64, error)
	GetByID(ctx context.Context, id uuid.UUID) (*notification.Notification, error)
	MarkRead(ctx context.Context, id, tenantID string) error
	MarkAllRead(ctx context.Context, tenantID string) error
}

// notificationDispatcher is the private interface NotificationService depends on
// for delivery — avoids coupling to the concrete Dispatcher type.
type notificationDispatcher interface {
	Dispatch(ctx context.Context, n *notification.Notification) error
}

// NotificationService handles notification lifecycle.
type NotificationService struct {
	repo              NotificationRepository
	dispatcher        notificationDispatcher
	logger            *slog.Logger
	fallbackRecipient string
}

// NewNotificationService creates a new NotificationService.
func NewNotificationService(repo NotificationRepository, dispatcher notificationDispatcher, logger *slog.Logger, fallbackRecipient string) *NotificationService {
	return &NotificationService{repo: repo, dispatcher: dispatcher, logger: logger, fallbackRecipient: fallbackRecipient}
}

// SendNotificationInput holds parameters for a manual notification send.
type SendNotificationInput struct {
	WorkspaceID string
	ChannelType string
	Recipient   string
	Subject     string
	Body        string
}

// Send creates, persists, and asynchronously dispatches a notification.
func (s *NotificationService) Send(ctx context.Context, in SendNotificationInput) (*notification.Notification, error) {
	wsID, err := uuid.Parse(in.WorkspaceID)
	if err != nil {
		return nil, notification.ErrInvalidWorkspace
	}

	n := notification.NewNotification(wsID, notification.NotificationChannelType(in.ChannelType), in.Recipient, in.Subject, in.Body)

	if err := s.repo.Save(ctx, n); err != nil {
		return nil, err
	}

	// Use context.WithoutCancel so the async dispatch is not cancelled when the
	// caller's request context ends, while still inheriting deadline/values.
	go s.dispatch(context.WithoutCancel(ctx), n)
	return n, nil
}

// GetNotification fetches a notification by its ID string.
func (s *NotificationService) GetNotification(ctx context.Context, id string) (*notification.Notification, error) {
	uid, err := uuid.Parse(id)
	if err != nil {
		return nil, notification.ErrInvalidNotificationID
	}
	return s.repo.GetByID(ctx, uid)
}

// ListNotifications returns paginated notifications for a workspace.
func (s *NotificationService) ListNotifications(ctx context.Context, workspaceID string, limit, offset int) ([]*notification.Notification, int64, error) {
	wsID, err := uuid.Parse(workspaceID)
	if err != nil {
		return nil, 0, notification.ErrInvalidWorkspace
	}
	return s.repo.ListByWorkspace(ctx, wsID, limit, offset)
}

// MarkNotificationRead marks a single notification as read.
func (s *NotificationService) MarkNotificationRead(ctx context.Context, id, tenantID string) error {
	if err := s.repo.MarkRead(ctx, id, tenantID); err != nil {
		return fmt.Errorf("MarkNotificationRead: %w", err)
	}
	return nil
}

// MarkAllNotificationsRead marks all unread notifications for a tenant as read.
func (s *NotificationService) MarkAllNotificationsRead(ctx context.Context, tenantID string) error {
	if err := s.repo.MarkAllRead(ctx, tenantID); err != nil {
		return fmt.Errorf("MarkAllNotificationsRead: %w", err)
	}
	return nil
}

// SendAlertNotification creates and dispatches a notification from a triggered alert event.
func (s *NotificationService) SendAlertNotification(ctx context.Context, evt *alert.AlertEvent) error {
	body, err := json.Marshal(map[string]interface{}{
		"channel_id":   evt.ChannelID.String(),
		"field":        evt.FieldName,
		"actual_value": evt.ActualValue,
		"threshold":    evt.Threshold,
		"condition":    string(evt.Condition),
		"severity":     string(evt.Severity),
		"message":      evt.Message,
	})
	if err != nil {
		return fmt.Errorf("marshal alert notification body: %w", err)
	}

	subject := fmt.Sprintf("[%s] Alert: %s %s %.2f (actual: %.2f)",
		evt.Severity, evt.FieldName, evt.Condition, evt.Threshold, evt.ActualValue)

	n := notification.NewNotification(evt.WorkspaceID, notification.ChannelTypeEmail, s.fallbackRecipient, subject, string(body))

	if err := s.repo.Save(ctx, n); err != nil {
		return err
	}

	go s.dispatch(context.WithoutCancel(ctx), n)
	return nil
}

// dispatch sends a notification and persists the outcome.
// Errors are logged; the notification status is updated in the DB regardless.
func (s *NotificationService) dispatch(ctx context.Context, n *notification.Notification) {
	if err := s.dispatcher.Dispatch(ctx, n); err != nil {
		n.MarkFailed(err.Error())
		s.logger.Error("dispatch notification failed",
			"notification_id", n.ID,
			"channel_type", n.ChannelType,
			"error", err,
		)
	} else {
		n.MarkSent()
	}
	if err := s.repo.Update(ctx, n); err != nil {
		s.logger.Error("update notification status failed",
			"notification_id", n.ID,
			"error", err,
		)
	}
}
