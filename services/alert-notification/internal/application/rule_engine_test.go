package application

import (
	"context"
	"log/slog"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/greenlab/alert-notification/internal/domain/alert"
)

// mockRuleRepo and mockAlertPublisher are defined in alert_service_test.go.

func newTestRuleEngine(t *testing.T) (*RuleEngine, *mockRuleRepo, *mockAlertPublisher) {
	t.Helper()
	repo := &mockRuleRepo{}
	pub := &mockAlertPublisher{}
	engine := NewRuleEngine(repo, pub, slog.Default())
	return engine, repo, pub
}

func TestLoadRules(t *testing.T) {
	ctx := context.Background()

	t.Run("success", func(t *testing.T) {
		engine, repo, _ := newTestRuleEngine(t)
		chID := uuid.New()
		rules := []*alert.Rule{
			{ID: uuid.New(), ChannelID: chID, Enabled: true},
			{ID: uuid.New(), ChannelID: chID, Enabled: true},
		}
		repo.On("ListEnabled", ctx).Return(rules, nil)

		err := engine.LoadRules(ctx)
		require.NoError(t, err)
		engine.mu.RLock()
		assert.Len(t, engine.rules[chID.String()], 2)
		engine.mu.RUnlock()
		repo.AssertExpectations(t)
	})

	t.Run("repo error propagated", func(t *testing.T) {
		engine, repo, _ := newTestRuleEngine(t)
		repo.On("ListEnabled", ctx).Return(nil, assert.AnError)

		err := engine.LoadRules(ctx)
		assert.Error(t, err)
	})
}

func TestEvaluate_TriggersAlert(t *testing.T) {
	ctx := context.Background()
	engine, repo, pub := newTestRuleEngine(t)

	chID := uuid.New()
	wsID := uuid.New()
	rule := &alert.Rule{
		ID:          uuid.New(),
		ChannelID:   chID,
		WorkspaceID: wsID,
		FieldName:   "temperature",
		Condition:   alert.ConditionGT,
		Threshold:   80.0,
		Severity:    alert.SeverityWarning,
		Enabled:     true,
		CooldownSec: 300,
	}
	repo.On("ListEnabled", ctx).Return([]*alert.Rule{rule}, nil)
	require.NoError(t, engine.LoadRules(ctx))

	pub.On("PublishAlert", ctx, mock.MatchedBy(func(e *alert.AlertEvent) bool {
		return e.RuleID == rule.ID && e.ActualValue == 95.0
	})).Return(nil)

	engine.Evaluate(ctx, TelemetryEvent{
		ChannelID: chID.String(),
		Fields:    map[string]float64{"temperature": 95.0},
		Timestamp: time.Now(),
	})

	pub.AssertExpectations(t)
}

func TestEvaluate_BelowThreshold_NoAlert(t *testing.T) {
	ctx := context.Background()
	engine, repo, pub := newTestRuleEngine(t)

	chID := uuid.New()
	rule := &alert.Rule{
		ID:          uuid.New(),
		ChannelID:   chID,
		FieldName:   "temperature",
		Condition:   alert.ConditionGT,
		Threshold:   80.0,
		Enabled:     true,
		CooldownSec: 300,
	}
	repo.On("ListEnabled", ctx).Return([]*alert.Rule{rule}, nil)
	require.NoError(t, engine.LoadRules(ctx))

	engine.Evaluate(ctx, TelemetryEvent{
		ChannelID: chID.String(),
		Fields:    map[string]float64{"temperature": 50.0},
		Timestamp: time.Now(),
	})

	pub.AssertNotCalled(t, "PublishAlert")
}

func TestEvaluate_CooldownPreventsDoubleAlerts(t *testing.T) {
	ctx := context.Background()
	engine, repo, pub := newTestRuleEngine(t)

	chID := uuid.New()
	rule := &alert.Rule{
		ID:          uuid.New(),
		ChannelID:   chID,
		FieldName:   "temperature",
		Condition:   alert.ConditionGT,
		Threshold:   80.0,
		Enabled:     true,
		CooldownSec: 300,
	}
	repo.On("ListEnabled", ctx).Return([]*alert.Rule{rule}, nil)
	require.NoError(t, engine.LoadRules(ctx))

	// First evaluation fires the alert.
	pub.On("PublishAlert", ctx, mock.Anything).Return(nil).Once()

	evt := TelemetryEvent{ChannelID: chID.String(), Fields: map[string]float64{"temperature": 95.0}, Timestamp: time.Now()}
	engine.Evaluate(ctx, evt)
	engine.Evaluate(ctx, evt) // second evaluation blocked by cooldown

	pub.AssertNumberOfCalls(t, "PublishAlert", 1)
}

func TestEvaluate_CooldownExpired_AlertFiredAgain(t *testing.T) {
	ctx := context.Background()
	engine, repo, pub := newTestRuleEngine(t)

	chID := uuid.New()
	rule := &alert.Rule{
		ID:          uuid.New(),
		ChannelID:   chID,
		FieldName:   "temperature",
		Condition:   alert.ConditionGT,
		Threshold:   80.0,
		Enabled:     true,
		CooldownSec: 0, // zero cooldown — always fires
	}
	repo.On("ListEnabled", ctx).Return([]*alert.Rule{rule}, nil)
	require.NoError(t, engine.LoadRules(ctx))

	pub.On("PublishAlert", ctx, mock.Anything).Return(nil)

	evt := TelemetryEvent{ChannelID: chID.String(), Fields: map[string]float64{"temperature": 95.0}, Timestamp: time.Now()}
	engine.Evaluate(ctx, evt)
	engine.Evaluate(ctx, evt)

	pub.AssertNumberOfCalls(t, "PublishAlert", 2)
}

func TestEvaluate_FieldNotInEvent_NoAlert(t *testing.T) {
	ctx := context.Background()
	engine, repo, pub := newTestRuleEngine(t)

	chID := uuid.New()
	rule := &alert.Rule{
		ID:          uuid.New(),
		ChannelID:   chID,
		FieldName:   "pressure", // event only has temperature
		Condition:   alert.ConditionGT,
		Threshold:   1.0,
		Enabled:     true,
		CooldownSec: 300,
	}
	repo.On("ListEnabled", ctx).Return([]*alert.Rule{rule}, nil)
	require.NoError(t, engine.LoadRules(ctx))

	engine.Evaluate(ctx, TelemetryEvent{
		ChannelID: chID.String(),
		Fields:    map[string]float64{"temperature": 95.0},
		Timestamp: time.Now(),
	})

	pub.AssertNotCalled(t, "PublishAlert")
}

func TestEvaluate_NoRulesForChannel_NoAlert(t *testing.T) {
	ctx := context.Background()
	engine, repo, pub := newTestRuleEngine(t)

	repo.On("ListEnabled", ctx).Return([]*alert.Rule{}, nil)
	require.NoError(t, engine.LoadRules(ctx))

	engine.Evaluate(ctx, TelemetryEvent{
		ChannelID: uuid.New().String(),
		Fields:    map[string]float64{"temperature": 999.0},
		Timestamp: time.Now(),
	})

	pub.AssertNotCalled(t, "PublishAlert")
}
