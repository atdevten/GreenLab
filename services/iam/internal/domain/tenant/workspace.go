package tenant

import (
	"fmt"
	"time"

	"github.com/google/uuid"
)

type Workspace struct {
	ID          uuid.UUID
	OrgID       uuid.UUID
	Name        string
	Slug        string
	Description string
	MemberCount int
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

func NewWorkspace(orgID uuid.UUID, name, slug, description string) (*Workspace, error) {
	if name == "" {
		return nil, fmt.Errorf("NewWorkspace: %w", ErrInvalidName)
	}
	if !isValidSlug(slug) {
		return nil, fmt.Errorf("NewWorkspace: %w", ErrInvalidSlug)
	}
	now := time.Now().UTC()
	return &Workspace{
		ID:          uuid.New(),
		OrgID:       orgID,
		Name:        name,
		Slug:        slug,
		Description: description,
		CreatedAt:   now,
		UpdatedAt:   now,
	}, nil
}
