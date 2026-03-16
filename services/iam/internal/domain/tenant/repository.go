package tenant

import (
	"context"

	"github.com/google/uuid"
)

type OrgRepository interface {
	Create(ctx context.Context, org *Org) error
	GetByID(ctx context.Context, id uuid.UUID) (*Org, error)
	GetBySlug(ctx context.Context, slug string) (*Org, error)
	List(ctx context.Context, limit, offset int) ([]*Org, int64, error)
	Update(ctx context.Context, org *Org) error
	Delete(ctx context.Context, id uuid.UUID) error
}

type WorkspaceRepository interface {
	Create(ctx context.Context, ws *Workspace) error
	GetByID(ctx context.Context, id uuid.UUID) (*Workspace, error)
	ListByOrg(ctx context.Context, orgID uuid.UUID) ([]*Workspace, error)
	Update(ctx context.Context, ws *Workspace) error
	Delete(ctx context.Context, id uuid.UUID) error
	ListMembers(ctx context.Context, workspaceID string) ([]WorkspaceMember, error)
	AddMember(ctx context.Context, m WorkspaceMember) error
	UpdateMember(ctx context.Context, workspaceID, userID, role string) error
	RemoveMember(ctx context.Context, workspaceID, userID string) error
}

type APIKeyRepository interface {
	ListAPIKeys(ctx context.Context, tenantID string) ([]APIKey, error)
	CreateAPIKey(ctx context.Context, key APIKey) error
	DeleteAPIKey(ctx context.Context, id, tenantID string) error
}
