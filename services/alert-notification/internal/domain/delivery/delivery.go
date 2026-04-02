package delivery

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// Log records a single webhook delivery attempt.
type Log struct {
	ID           uuid.UUID
	RuleID       uuid.UUID
	URL          string
	HTTPStatus   int
	LatencyMS    int64
	ResponseBody string
	ErrorMsg     string
	DeliveredAt  time.Time
}

// Repository persists and retrieves delivery logs.
type Repository interface {
	Save(ctx context.Context, l *Log) error
	ListByRule(ctx context.Context, ruleID uuid.UUID, limit, offset int) ([]*Log, int64, error)
}
