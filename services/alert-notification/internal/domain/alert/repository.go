package alert

import (
	"context"

	"github.com/google/uuid"
)

type RuleRepository interface {
	Create(ctx context.Context, r *Rule) error
	GetByID(ctx context.Context, id uuid.UUID) (*Rule, error)
	ListByChannel(ctx context.Context, channelID uuid.UUID) ([]*Rule, error)
	ListByWorkspace(ctx context.Context, workspaceID uuid.UUID, limit, offset int) ([]*Rule, int64, error)
	Update(ctx context.Context, r *Rule) error
	Delete(ctx context.Context, id uuid.UUID) error
	ListEnabled(ctx context.Context) ([]*Rule, error)
}
