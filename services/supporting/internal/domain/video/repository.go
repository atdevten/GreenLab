package video

import (
	"context"

	"github.com/google/uuid"
)

type StreamRepository interface {
	Create(ctx context.Context, s *Stream) error
	GetByID(ctx context.Context, id uuid.UUID) (*Stream, error)
	ListByDevice(ctx context.Context, deviceID uuid.UUID, limit, offset int) ([]*Stream, int64, error)
	ListByWorkspace(ctx context.Context, workspaceID uuid.UUID, limit, offset int) ([]*Stream, int64, error)
	Update(ctx context.Context, s *Stream) error
	Delete(ctx context.Context, id uuid.UUID) error
}
