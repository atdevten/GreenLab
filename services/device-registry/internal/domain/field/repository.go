package field

import (
	"context"

	"github.com/google/uuid"
)

type FieldRepository interface {
	Create(ctx context.Context, f *Field) error
	GetByID(ctx context.Context, id uuid.UUID) (*Field, error)
	ListByChannel(ctx context.Context, channelID uuid.UUID) ([]*Field, error)
	Update(ctx context.Context, f *Field) error
	Delete(ctx context.Context, id uuid.UUID) error
}
