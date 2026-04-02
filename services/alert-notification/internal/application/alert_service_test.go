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
	"github.com/greenlab/alert-notification/internal/domain/delivery"
)

// --- mock RuleRepository ---

type mockRuleRepo struct{ mock.Mock }

func (m *mockRuleRepo) Create(ctx context.Context, rule *alert.Rule) error {
	return m.Called(ctx, rule).Error(0)
}
func (m *mockRuleRepo) GetByID(ctx context.Context, id uuid.UUID) (*alert.Rule, error) {
	args := m.Called(ctx, id)
	r, _ := args.Get(0).(*alert.Rule)
	return r, args.Error(1)
}
func (m *mockRuleRepo) ListByChannel(ctx context.Context, channelID uuid.UUID) ([]*alert.Rule, error) {
	args := m.Called(ctx, channelID)
	r, _ := args.Get(0).([]*alert.Rule)
	return r, args.Error(1)
}
func (m *mockRuleRepo) ListByWorkspace(ctx context.Context, workspaceID uuid.UUID, limit, offset int) ([]*alert.Rule, int64, error) {
	args := m.Called(ctx, workspaceID, limit, offset)
	r, _ := args.Get(0).([]*alert.Rule)
	return r, args.Get(1).(int64), args.Error(2)
}
func (m *mockRuleRepo) Update(ctx context.Context, rule *alert.Rule) error {
	return m.Called(ctx, rule).Error(0)
}
func (m *mockRuleRepo) Delete(ctx context.Context, id uuid.UUID) error {
	return m.Called(ctx, id).Error(0)
}
func (m *mockRuleRepo) ListEnabled(ctx context.Context) ([]*alert.Rule, error) {
	args := m.Called(ctx)
	r, _ := args.Get(0).([]*alert.Rule)
	return r, args.Error(1)
}

// --- mock AlertPublisher ---

type mockAlertPublisher struct{ mock.Mock }

func (m *mockAlertPublisher) PublishAlert(ctx context.Context, event *alert.AlertEvent) error {
	return m.Called(ctx, event).Error(0)
}

// --- mock DeliveryRepository ---

type mockDeliveryRepo struct{ mock.Mock }

func (m *mockDeliveryRepo) Save(ctx context.Context, l *delivery.Log) error {
	return m.Called(ctx, l).Error(0)
}

func (m *mockDeliveryRepo) ListByRule(ctx context.Context, ruleID uuid.UUID, limit, offset int) ([]*delivery.Log, int64, error) {
	args := m.Called(ctx, ruleID, limit, offset)
	logs, _ := args.Get(0).([]*delivery.Log)
	return logs, args.Get(1).(int64), args.Error(2)
}

// --- helpers ---

func newTestAlertService(t *testing.T) (*AlertService, *mockRuleRepo, *mockAlertPublisher) {
	t.Helper()
	repo := &mockRuleRepo{}
	pub := &mockAlertPublisher{}
	deliveryRepo := &mockDeliveryRepo{}
	svc := NewAlertService(repo, pub, deliveryRepo, slog.Default())
	return svc, repo, pub
}

func newTestAlertServiceWithDelivery(t *testing.T) (*AlertService, *mockRuleRepo, *mockAlertPublisher, *mockDeliveryRepo) {
	t.Helper()
	repo := &mockRuleRepo{}
	pub := &mockAlertPublisher{}
	deliveryRepo := &mockDeliveryRepo{}
	svc := NewAlertService(repo, pub, deliveryRepo, slog.Default())
	return svc, repo, pub, deliveryRepo
}

// --- tests ---

func TestCreateRule(t *testing.T) {
	ctx := context.Background()

	t.Run("success", func(t *testing.T) {
		svc, repo, _ := newTestAlertService(t)
		repo.On("Create", ctx, mock.AnythingOfType("*alert.Rule")).Return(nil)

		rule, err := svc.CreateRule(ctx, CreateRuleInput{
			ChannelID:   uuid.New().String(),
			WorkspaceID: uuid.New().String(),
			Name:        "High Temp",
			FieldName:   "temperature",
			Condition:   "gt",
			Threshold:   80.0,
			Severity:    "warning",
		})
		require.NoError(t, err)
		assert.Equal(t, "High Temp", rule.Name)
		repo.AssertExpectations(t)
	})

	t.Run("invalid channel_id returns ErrInvalidChannelID", func(t *testing.T) {
		svc, _, _ := newTestAlertService(t)
		_, err := svc.CreateRule(ctx, CreateRuleInput{
			ChannelID: "not-uuid", WorkspaceID: uuid.New().String(),
			Name: "r", FieldName: "f", Condition: "gt", Severity: "info",
		})
		assert.ErrorIs(t, err, alert.ErrInvalidChannelID)
	})

	t.Run("invalid workspace_id returns ErrInvalidWorkspace", func(t *testing.T) {
		svc, _, _ := newTestAlertService(t)
		_, err := svc.CreateRule(ctx, CreateRuleInput{
			ChannelID: uuid.New().String(), WorkspaceID: "bad",
			Name: "r", FieldName: "f", Condition: "gt", Severity: "info",
		})
		assert.ErrorIs(t, err, alert.ErrInvalidWorkspace)
	})

	t.Run("custom cooldown is applied", func(t *testing.T) {
		svc, repo, _ := newTestAlertService(t)
		repo.On("Create", ctx, mock.MatchedBy(func(r *alert.Rule) bool {
			return r.CooldownSec == 120
		})).Return(nil)

		_, err := svc.CreateRule(ctx, CreateRuleInput{
			ChannelID: uuid.New().String(), WorkspaceID: uuid.New().String(),
			Name: "r", FieldName: "f", Condition: "gt", Severity: "info",
			CooldownSec: 120,
		})
		require.NoError(t, err)
		repo.AssertExpectations(t)
	})

	t.Run("repo error is propagated", func(t *testing.T) {
		svc, repo, _ := newTestAlertService(t)
		dbErr := errors.New("db error")
		repo.On("Create", ctx, mock.AnythingOfType("*alert.Rule")).Return(dbErr)

		_, err := svc.CreateRule(ctx, CreateRuleInput{
			ChannelID: uuid.New().String(), WorkspaceID: uuid.New().String(),
			Name: "r", FieldName: "f", Condition: "gt", Severity: "info",
		})
		assert.ErrorIs(t, err, dbErr)
		repo.AssertExpectations(t)
	})
}

func TestGetRule(t *testing.T) {
	ctx := context.Background()

	t.Run("success", func(t *testing.T) {
		svc, repo, _ := newTestAlertService(t)
		id := uuid.New()
		expected := &alert.Rule{ID: id, Name: "r"}
		repo.On("GetByID", ctx, id).Return(expected, nil)

		rule, err := svc.GetRule(ctx, id.String())
		require.NoError(t, err)
		assert.Equal(t, expected, rule)
		repo.AssertExpectations(t)
	})

	t.Run("invalid id returns ErrInvalidRuleID", func(t *testing.T) {
		svc, _, _ := newTestAlertService(t)
		_, err := svc.GetRule(ctx, "not-a-uuid")
		assert.ErrorIs(t, err, alert.ErrInvalidRuleID)
	})

	t.Run("not found returns ErrRuleNotFound", func(t *testing.T) {
		svc, repo, _ := newTestAlertService(t)
		id := uuid.New()
		repo.On("GetByID", ctx, id).Return(nil, alert.ErrRuleNotFound)

		_, err := svc.GetRule(ctx, id.String())
		assert.ErrorIs(t, err, alert.ErrRuleNotFound)
		repo.AssertExpectations(t)
	})
}

func TestListRules(t *testing.T) {
	ctx := context.Background()

	t.Run("success", func(t *testing.T) {
		svc, repo, _ := newTestAlertService(t)
		wsID := uuid.New()
		expected := []*alert.Rule{{Name: "r1"}, {Name: "r2"}}
		repo.On("ListByWorkspace", ctx, wsID, 10, 0).Return(expected, int64(2), nil)

		rules, total, err := svc.ListRules(ctx, wsID.String(), 10, 0)
		require.NoError(t, err)
		assert.Equal(t, int64(2), total)
		assert.Len(t, rules, 2)
		repo.AssertExpectations(t)
	})

	t.Run("invalid workspace_id returns ErrInvalidWorkspace", func(t *testing.T) {
		svc, _, _ := newTestAlertService(t)
		_, _, err := svc.ListRules(ctx, "bad", 10, 0)
		assert.ErrorIs(t, err, alert.ErrInvalidWorkspace)
	})
}

func TestUpdateRule(t *testing.T) {
	ctx := context.Background()

	t.Run("success", func(t *testing.T) {
		svc, repo, _ := newTestAlertService(t)
		id := uuid.New()
		existing := &alert.Rule{ID: id, Name: "old", Threshold: 50.0, Severity: alert.SeverityInfo, Enabled: true}
		newThreshold := 99.0
		newEnabled := false

		repo.On("GetByID", ctx, id).Return(existing, nil)
		repo.On("Update", ctx, mock.MatchedBy(func(r *alert.Rule) bool {
			return r.Name == "new name" && r.Threshold == 99.0 && !r.Enabled
		})).Return(nil)

		rule, err := svc.UpdateRule(ctx, id.String(), UpdateRuleInput{
			Name:      "new name",
			Threshold: &newThreshold,
			Enabled:   &newEnabled,
		})
		require.NoError(t, err)
		assert.Equal(t, "new name", rule.Name)
		assert.Equal(t, 99.0, rule.Threshold)
		assert.False(t, rule.Enabled)
		repo.AssertExpectations(t)
	})

	t.Run("invalid id returns ErrInvalidRuleID", func(t *testing.T) {
		svc, _, _ := newTestAlertService(t)
		_, err := svc.UpdateRule(ctx, "bad", UpdateRuleInput{})
		assert.ErrorIs(t, err, alert.ErrInvalidRuleID)
	})

	t.Run("not found returns ErrRuleNotFound", func(t *testing.T) {
		svc, repo, _ := newTestAlertService(t)
		id := uuid.New()
		repo.On("GetByID", ctx, id).Return(nil, alert.ErrRuleNotFound)

		_, err := svc.UpdateRule(ctx, id.String(), UpdateRuleInput{Name: "x"})
		assert.ErrorIs(t, err, alert.ErrRuleNotFound)
		repo.AssertExpectations(t)
	})
}

func TestDeleteRule(t *testing.T) {
	ctx := context.Background()

	t.Run("success", func(t *testing.T) {
		svc, repo, _ := newTestAlertService(t)
		id := uuid.New()
		repo.On("Delete", ctx, id).Return(nil)

		err := svc.DeleteRule(ctx, id.String())
		require.NoError(t, err)
		repo.AssertExpectations(t)
	})

	t.Run("invalid id returns ErrInvalidRuleID", func(t *testing.T) {
		svc, _, _ := newTestAlertService(t)
		err := svc.DeleteRule(ctx, "not-a-uuid")
		assert.ErrorIs(t, err, alert.ErrInvalidRuleID)
	})
}

func TestListDeliveries(t *testing.T) {
	ctx := context.Background()

	t.Run("success returns paginated delivery logs", func(t *testing.T) {
		svc, _, _, deliveryRepo := newTestAlertServiceWithDelivery(t)
		ruleID := uuid.New()
		expected := []*delivery.Log{
			{ID: uuid.New(), RuleID: ruleID, URL: "https://example.com/hook", HTTPStatus: 200, LatencyMS: 42},
			{ID: uuid.New(), RuleID: ruleID, URL: "https://example.com/hook", HTTPStatus: 500, LatencyMS: 150, ErrorMsg: "server error"},
		}
		deliveryRepo.On("ListByRule", ctx, ruleID, 10, 0).Return(expected, int64(2), nil)

		logs, total, err := svc.ListDeliveries(ctx, ruleID.String(), 10, 0)
		require.NoError(t, err)
		assert.Equal(t, int64(2), total)
		assert.Len(t, logs, 2)
		assert.Equal(t, 200, logs[0].HTTPStatus)
		assert.Equal(t, int64(42), logs[0].LatencyMS)
		deliveryRepo.AssertExpectations(t)
	})

	t.Run("invalid rule_id returns ErrInvalidRuleID", func(t *testing.T) {
		svc, _, _, _ := newTestAlertServiceWithDelivery(t)
		_, _, err := svc.ListDeliveries(ctx, "not-a-uuid", 10, 0)
		assert.ErrorIs(t, err, alert.ErrInvalidRuleID)
	})

	t.Run("repo error is propagated", func(t *testing.T) {
		svc, _, _, deliveryRepo := newTestAlertServiceWithDelivery(t)
		ruleID := uuid.New()
		dbErr := errors.New("db error")
		deliveryRepo.On("ListByRule", ctx, ruleID, 10, 0).Return(nil, int64(0), dbErr)

		_, _, err := svc.ListDeliveries(ctx, ruleID.String(), 10, 0)
		assert.ErrorIs(t, err, dbErr)
		deliveryRepo.AssertExpectations(t)
	})

	t.Run("empty result returns empty slice", func(t *testing.T) {
		svc, _, _, deliveryRepo := newTestAlertServiceWithDelivery(t)
		ruleID := uuid.New()
		deliveryRepo.On("ListByRule", ctx, ruleID, 10, 0).Return([]*delivery.Log{}, int64(0), nil)

		logs, total, err := svc.ListDeliveries(ctx, ruleID.String(), 10, 0)
		require.NoError(t, err)
		assert.Equal(t, int64(0), total)
		assert.Empty(t, logs)
		deliveryRepo.AssertExpectations(t)
	})
}

func TestVerifyWebhookSignature(t *testing.T) {
	ctx := context.Background()

	t.Run("valid signature returns true", func(t *testing.T) {
		svc, repo, _ := newTestAlertService(t)
		id := uuid.New()
		secret := "super-secret"
		payload := `{"sensor":"temperature","value":42}`
		sig := "sha256=" + alert.ComputeHMAC(secret, payload)

		repo.On("GetByID", ctx, id).Return(&alert.Rule{ID: id, WebhookSecret: secret}, nil)

		valid, err := svc.VerifyWebhookSignature(ctx, id.String(), payload, sig)
		require.NoError(t, err)
		assert.True(t, valid)
		repo.AssertExpectations(t)
	})

	t.Run("wrong signature returns false", func(t *testing.T) {
		svc, repo, _ := newTestAlertService(t)
		id := uuid.New()
		secret := "super-secret"
		repo.On("GetByID", ctx, id).Return(&alert.Rule{ID: id, WebhookSecret: secret}, nil)

		valid, err := svc.VerifyWebhookSignature(ctx, id.String(), `{"v":1}`, "sha256=wronghex")
		require.NoError(t, err)
		assert.False(t, valid)
		repo.AssertExpectations(t)
	})

	t.Run("invalid rule id returns ErrInvalidRuleID", func(t *testing.T) {
		svc, _, _ := newTestAlertService(t)
		_, err := svc.VerifyWebhookSignature(ctx, "not-a-uuid", "payload", "sig")
		assert.ErrorIs(t, err, alert.ErrInvalidRuleID)
	})

	t.Run("rule not found returns ErrRuleNotFound", func(t *testing.T) {
		svc, repo, _ := newTestAlertService(t)
		id := uuid.New()
		repo.On("GetByID", ctx, id).Return(nil, alert.ErrRuleNotFound)

		_, err := svc.VerifyWebhookSignature(ctx, id.String(), "payload", "sig")
		assert.ErrorIs(t, err, alert.ErrRuleNotFound)
		repo.AssertExpectations(t)
	})

	t.Run("no secret configured returns ErrNoWebhookSecret", func(t *testing.T) {
		svc, repo, _ := newTestAlertService(t)
		id := uuid.New()
		repo.On("GetByID", ctx, id).Return(&alert.Rule{ID: id, WebhookSecret: ""}, nil)

		_, err := svc.VerifyWebhookSignature(ctx, id.String(), "payload", "sig")
		assert.ErrorIs(t, err, alert.ErrNoWebhookSecret)
		repo.AssertExpectations(t)
	})
}
