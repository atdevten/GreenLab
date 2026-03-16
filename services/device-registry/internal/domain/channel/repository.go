package channel

import (
	"context"

	"github.com/google/uuid"
)

type ChannelRepository interface {
	Create(ctx context.Context, ch *Channel) error
	GetByID(ctx context.Context, id uuid.UUID) (*Channel, error)
	ListByWorkspace(ctx context.Context, workspaceID uuid.UUID, limit, offset int) ([]*Channel, int64, error)
	ListByDevice(ctx context.Context, deviceID uuid.UUID, limit, offset int) ([]*Channel, int64, error)
	Update(ctx context.Context, ch *Channel) error
	Delete(ctx context.Context, id uuid.UUID) error
}
