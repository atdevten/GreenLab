package application

import (
	"context"
	"errors"
	"log/slog"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/greenlab/alert-notification/internal/domain/alert"
	"github.com/greenlab/alert-notification/internal/domain/notification"
)

// --- mock NotificationRepository ---

type mockNotifRepo struct{ mock.Mock }

func (m *mockNotifRepo) Save(ctx context.Context, n *notification.Notification) error {
	return m.Called(ctx, n).Error(0)
}
func (m *mockNotifRepo) Update(ctx context.Context, n *notification.Notification) error {
	return m.Called(ctx, n).Error(0)
}
func (m *mockNotifRepo) GetByID(ctx context.Context, id uuid.UUID) (*notification.Notification, error) {
	args := m.Called(ctx, id)
	n, _ := args.Get(0).(*notification.Notification)
	return n, args.Error(1)
}
func (m *mockNotifRepo) ListByWorkspace(ctx context.Context, workspaceID uuid.UUID, limit, offset int) ([]*notification.Notification, int64, error) {
	args := m.Called(ctx, workspaceID, limit, offset)
	n, _ := args.Get(0).([]*notification.Notification)
	return n, args.Get(1).(int64), args.Error(2)
}
func (m *mockNotifRepo) MarkRead(ctx context.Context, id, tenantID string) error {
	return m.Called(ctx, id, tenantID).Error(0)
}
func (m *mockNotifRepo) MarkAllRead(ctx context.Context, tenantID string) error {
	return m.Called(ctx, tenantID).Error(0)
}

// --- mock notificationDispatcher ---

type mockDispatcher struct{ mock.Mock }

func (m *mockDispatcher) Dispatch(ctx context.Context, n *notification.Notification) error {
	return m.Called(ctx, n).Error(0)
}

// --- helpers ---

func newTestNotifService(t *testing.T) (*NotificationService, *mockNotifRepo, *mockDispatcher) {
	t.Helper()
	repo := &mockNotifRepo{}
	d := &mockDispatcher{}
	svc := NewNotificationService(repo, d, slog.Default(), "alerts@test.io")
	return svc, repo, d
}

// --- tests ---

func TestSend(t *testing.T) {
	ctx := context.Background()

	t.Run("success — saves and returns pending notification", func(t *testing.T) {
		svc, repo, d := newTestNotifService(t)
		wsID := uuid.New()

		repo.On("Save", ctx, mock.AnythingOfType("*notification.Notification")).Return(nil)
		// dispatch goroutine may run after the test; permit but don't require calls
		d.On("Dispatch", mock.Anything, mock.AnythingOfType("*notification.Notification")).Maybe().Return(nil)
		repo.On("Update", mock.Anything, mock.AnythingOfType("*notification.Notification")).Maybe().Return(nil)

		n, err := svc.Send(ctx, SendNotificationInput{
			WorkspaceID: wsID.String(),
			ChannelType: "email",
			Recipient:   "user@example.com",
			Subject:     "Test",
			Body:        "hello",
		})
		require.NoError(t, err)
		assert.Equal(t, notification.NotificationStatusPending, n.Status)
		assert.Equal(t, "user@example.com", n.Recipient)
		repo.AssertCalled(t, "Save", ctx, mock.AnythingOfType("*notification.Notification"))
	})

	t.Run("invalid workspace_id returns ErrInvalidWorkspace", func(t *testing.T) {
		svc, _, _ := newTestNotifService(t)
		_, err := svc.Send(ctx, SendNotificationInput{WorkspaceID: "bad"})
		assert.ErrorIs(t, err, notification.ErrInvalidWorkspace)
	})

	t.Run("repo save error is returned", func(t *testing.T) {
		svc, repo, _ := newTestNotifService(t)
		dbErr := errors.New("db error")
		repo.On("Save", ctx, mock.AnythingOfType("*notification.Notification")).Return(dbErr)

		_, err := svc.Send(ctx, SendNotificationInput{
			WorkspaceID: uuid.New().String(),
			ChannelType: "email",
			Recipient:   "r@example.com",
			Subject:     "s",
			Body:        "b",
		})
		assert.ErrorIs(t, err, dbErr)
		repo.AssertExpectations(t)
	})
}

func TestGetNotification(t *testing.T) {
	ctx := context.Background()

	t.Run("success", func(t *testing.T) {
		svc, repo, _ := newTestNotifService(t)
		id := uuid.New()
		expected := &notification.Notification{ID: id}
		repo.On("GetByID", ctx, id).Return(expected, nil)

		n, err := svc.GetNotification(ctx, id.String())
		require.NoError(t, err)
		assert.Equal(t, expected, n)
		repo.AssertExpectations(t)
	})

	t.Run("invalid id returns ErrInvalidNotificationID (not 404)", func(t *testing.T) {
		svc, _, _ := newTestNotifService(t)
		_, err := svc.GetNotification(ctx, "not-a-uuid")
		assert.ErrorIs(t, err, notification.ErrInvalidNotificationID)
		assert.NotErrorIs(t, err, notification.ErrNotificationNotFound)
	})

	t.Run("not found returns ErrNotificationNotFound", func(t *testing.T) {
		svc, repo, _ := newTestNotifService(t)
		id := uuid.New()
		repo.On("GetByID", ctx, id).Return(nil, notification.ErrNotificationNotFound)

		_, err := svc.GetNotification(ctx, id.String())
		assert.ErrorIs(t, err, notification.ErrNotificationNotFound)
		repo.AssertExpectations(t)
	})
}

func TestListNotifications(t *testing.T) {
	ctx := context.Background()

	t.Run("success", func(t *testing.T) {
		svc, repo, _ := newTestNotifService(t)
		wsID := uuid.New()
		expected := []*notification.Notification{{Subject: "a"}, {Subject: "b"}}
		repo.On("ListByWorkspace", ctx, wsID, 10, 0).Return(expected, int64(2), nil)

		items, total, err := svc.ListNotifications(ctx, wsID.String(), 10, 0)
		require.NoError(t, err)
		assert.Equal(t, int64(2), total)
		assert.Len(t, items, 2)
		repo.AssertExpectations(t)
	})

	t.Run("invalid workspace_id returns ErrInvalidWorkspace", func(t *testing.T) {
		svc, _, _ := newTestNotifService(t)
		_, _, err := svc.ListNotifications(ctx, "bad", 10, 0)
		assert.ErrorIs(t, err, notification.ErrInvalidWorkspace)
	})
}

func TestSendAlertNotification(t *testing.T) {
	ctx := context.Background()

	t.Run("success — saves with fallback recipient", func(t *testing.T) {
		svc, repo, d := newTestNotifService(t)

		repo.On("Save", ctx, mock.MatchedBy(func(n *notification.Notification) bool {
			return n.Recipient == "alerts@test.io" && n.ChannelType == notification.ChannelTypeEmail
		})).Return(nil)
		d.On("Dispatch", mock.Anything, mock.AnythingOfType("*notification.Notification")).Maybe().Return(nil)
		repo.On("Update", mock.Anything, mock.AnythingOfType("*notification.Notification")).Maybe().Return(nil)

		err := svc.SendAlertNotification(ctx, &alert.AlertEvent{
			ID:          uuid.New(),
			RuleID:      uuid.New(),
			ChannelID:   uuid.New(),
			WorkspaceID: uuid.New(),
			FieldName:   "temperature",
			ActualValue: 95.0,
			Threshold:   80.0,
			Condition:   alert.ConditionGT,
			Severity:    alert.SeverityWarning,
			Message:     "too hot",
			// TriggeredAt left as zero value
		})
		require.NoError(t, err)
		repo.AssertCalled(t, "Save", ctx, mock.MatchedBy(func(n *notification.Notification) bool {
			return n.Recipient == "alerts@test.io"
		}))
	})

	t.Run("repo save error is returned", func(t *testing.T) {
		svc, repo, _ := newTestNotifService(t)
		dbErr := errors.New("db error")
		repo.On("Save", ctx, mock.AnythingOfType("*notification.Notification")).Return(dbErr)

		err := svc.SendAlertNotification(ctx, &alert.AlertEvent{
			WorkspaceID: uuid.New(),
			FieldName:   "f",
			Condition:   alert.ConditionGT,
			Severity:    alert.SeverityInfo,
		})
		assert.ErrorIs(t, err, dbErr)
		repo.AssertExpectations(t)
	})
}

func TestDispatch_MarksSentOnSuccess(t *testing.T) {
	svc, repo, d := newTestNotifService(t)
	n := notification.NewNotification(uuid.New(), notification.ChannelTypeEmail, "r@example.com", "s", "b")

	d.On("Dispatch", mock.Anything, n).Return(nil)
	repo.On("Update", mock.Anything, mock.MatchedBy(func(u *notification.Notification) bool {
		return u.Status == notification.NotificationStatusSent
	})).Return(nil)

	svc.dispatch(context.Background(), n)

	assert.Equal(t, notification.NotificationStatusSent, n.Status)
	d.AssertExpectations(t)
	repo.AssertExpectations(t)
}

func TestDispatch_MarksFailedOnError(t *testing.T) {
	svc, repo, d := newTestNotifService(t)
	n := notification.NewNotification(uuid.New(), notification.ChannelTypeEmail, "r@example.com", "s", "b")
	dispatchErr := errors.New("smtp timeout")

	d.On("Dispatch", mock.Anything, n).Return(dispatchErr)
	repo.On("Update", mock.Anything, mock.MatchedBy(func(u *notification.Notification) bool {
		return u.Status == notification.NotificationStatusFailed && u.ErrorMsg == "smtp timeout" && u.Retries == 1
	})).Return(nil)

	svc.dispatch(context.Background(), n)

	assert.Equal(t, notification.NotificationStatusFailed, n.Status)
	d.AssertExpectations(t)
	repo.AssertExpectations(t)
}
