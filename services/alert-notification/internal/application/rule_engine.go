package application

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/greenlab/alert-notification/internal/domain/alert"
)

// TelemetryEvent represents an incoming telemetry reading for evaluation.
type TelemetryEvent struct {
	ChannelID string
	DeviceID  string
	Fields    map[string]float64
	Timestamp time.Time
}

// RuleEngine evaluates incoming telemetry events against active rules.
type RuleEngine struct {
	repo      alert.RuleRepository
	publisher AlertPublisher
	log       *slog.Logger

	mu          sync.RWMutex
	rules       map[string][]*alert.Rule // channelID → rules
	lastFired   map[string]time.Time     // ruleID → last fired time
	refreshedAt time.Time
}

// NewRuleEngine creates a new RuleEngine.
func NewRuleEngine(repo alert.RuleRepository, publisher AlertPublisher, log *slog.Logger) *RuleEngine {
	return &RuleEngine{
		repo:      repo,
		publisher: publisher,
		log:       log,
		rules:     make(map[string][]*alert.Rule),
		lastFired: make(map[string]time.Time),
	}
}

// LoadRules loads all enabled rules from the database into memory.
func (e *RuleEngine) LoadRules(ctx context.Context) error {
	rules, err := e.repo.ListEnabled(ctx)
	if err != nil {
		return err
	}

	newRules := make(map[string][]*alert.Rule)
	for _, r := range rules {
		chID := r.ChannelID.String()
		newRules[chID] = append(newRules[chID], r)
	}

	e.mu.Lock()
	e.rules = newRules
	e.refreshedAt = time.Now().UTC()
	e.mu.Unlock()

	e.log.Info("rules loaded", "count", len(rules))
	return nil
}

// Evaluate evaluates a telemetry event against all rules for the event's channel.
func (e *RuleEngine) Evaluate(ctx context.Context, evt TelemetryEvent) {
	e.mu.RLock()
	rules := e.rules[evt.ChannelID]
	e.mu.RUnlock()

	if len(rules) == 0 {
		return
	}

	for _, rule := range rules {
		if !rule.Enabled {
			continue
		}

		value, ok := evt.Fields[rule.FieldName]
		if !ok {
			continue
		}

		if !rule.Evaluate(value) {
			continue
		}

		// Check cooldown — hold write lock for the full check-and-set to avoid TOCTOU.
		ruleIDStr := rule.ID.String()
		e.mu.Lock()
		lastFired, hasFired := e.lastFired[ruleIDStr]
		if hasFired && time.Since(lastFired) < time.Duration(rule.CooldownSec)*time.Second {
			e.mu.Unlock()
			continue
		}
		// Reserve the slot optimistically before publishing; prevents double-firing
		// even when Evaluate is called concurrently.
		e.lastFired[ruleIDStr] = time.Now().UTC()
		e.mu.Unlock()

		alertEvt := &alert.AlertEvent{
			ID:          uuid.New(),
			RuleID:      rule.ID,
			ChannelID:   rule.ChannelID,
			WorkspaceID: rule.WorkspaceID,
			FieldName:   rule.FieldName,
			ActualValue: value,
			Threshold:   rule.Threshold,
			Condition:   rule.Condition,
			Severity:    rule.Severity,
			Message:     rule.Message,
			TriggeredAt: evt.Timestamp,
		}

		if err := e.publisher.PublishAlert(ctx, alertEvt); err != nil {
			e.log.Error("publish alert", "rule_id", ruleIDStr, "error", err)
		}
	}
}

// StartRuleRefresh periodically reloads rules from the database until ctx is cancelled.
func (e *RuleEngine) StartRuleRefresh(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := e.LoadRules(ctx); err != nil {
				e.log.Error("reload rules", "error", err)
			}
		}
	}
}
