package application

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/greenlab/iam/internal/domain/tenant"
)

type TenantService struct {
	orgRepo    tenant.OrgRepository
	wsRepo     tenant.WorkspaceRepository
	apiKeyRepo tenant.APIKeyRepository
}

func NewTenantService(orgRepo tenant.OrgRepository, wsRepo tenant.WorkspaceRepository, apiKeyRepo tenant.APIKeyRepository) *TenantService {
	return &TenantService{orgRepo: orgRepo, wsRepo: wsRepo, apiKeyRepo: apiKeyRepo}
}

type CreateOrgInput struct {
	Name        string
	Slug        string
	OwnerUserID string
}

func (s *TenantService) CreateOrg(ctx context.Context, in CreateOrgInput) (*tenant.Org, error) {
	ownerID, err := uuid.Parse(in.OwnerUserID)
	if err != nil {
		return nil, fmt.Errorf("CreateOrg.ParseOwnerID: %w", err)
	}
	existing, err := s.orgRepo.GetBySlug(ctx, in.Slug)
	if err != nil && !errors.Is(err, tenant.ErrOrgNotFound) {
		return nil, fmt.Errorf("CreateOrg.CheckSlug: %w", err)
	}
	if existing != nil {
		return nil, fmt.Errorf("CreateOrg.CheckSlug: %w", tenant.ErrSlugAlreadyTaken)
	}
	org, err := tenant.NewOrg(in.Name, in.Slug, ownerID)
	if err != nil {
		return nil, fmt.Errorf("CreateOrg.NewOrg: %w", err)
	}
	if err := s.orgRepo.Create(ctx, org); err != nil {
		return nil, fmt.Errorf("CreateOrg.Create: %w", err)
	}
	return org, nil
}

func (s *TenantService) GetOrg(ctx context.Context, id string) (*tenant.Org, error) {
	uid, err := uuid.Parse(id)
	if err != nil {
		return nil, fmt.Errorf("GetOrg.ParseID: %w", err)
	}
	org, err := s.orgRepo.GetByID(ctx, uid)
	if err != nil {
		return nil, fmt.Errorf("GetOrg.GetByID: %w", err)
	}
	return org, nil
}

func (s *TenantService) ListOrgs(ctx context.Context, limit, offset int) ([]*tenant.Org, int64, error) {
	orgs, total, err := s.orgRepo.List(ctx, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("ListOrgs.List: %w", err)
	}
	return orgs, total, nil
}

type UpdateOrgInput struct {
	Name    string
	LogoURL string
	Website string
}

func (s *TenantService) UpdateOrg(ctx context.Context, id string, in UpdateOrgInput) (*tenant.Org, error) {
	uid, err := uuid.Parse(id)
	if err != nil {
		return nil, fmt.Errorf("UpdateOrg.ParseID: %w", err)
	}
	org, err := s.orgRepo.GetByID(ctx, uid)
	if err != nil {
		return nil, fmt.Errorf("UpdateOrg.GetByID: %w", err)
	}
	if in.Name != "" {
		org.Name = in.Name
	}
	if in.LogoURL != "" {
		org.LogoURL = in.LogoURL
	}
	if in.Website != "" {
		org.Website = in.Website
	}
	org.UpdatedAt = time.Now().UTC()
	if err := s.orgRepo.Update(ctx, org); err != nil {
		return nil, fmt.Errorf("UpdateOrg.Update: %w", err)
	}
	return org, nil
}

func (s *TenantService) DeleteOrg(ctx context.Context, id string) error {
	uid, err := uuid.Parse(id)
	if err != nil {
		return fmt.Errorf("DeleteOrg.ParseID: %w", err)
	}
	if err := s.orgRepo.Delete(ctx, uid); err != nil {
		return fmt.Errorf("DeleteOrg.Delete: %w", err)
	}
	return nil
}

type CreateWorkspaceInput struct {
	OrgID       string
	Name        string
	Slug        string
	Description string
}

func (s *TenantService) CreateWorkspace(ctx context.Context, in CreateWorkspaceInput) (*tenant.Workspace, error) {
	orgID, err := uuid.Parse(in.OrgID)
	if err != nil {
		return nil, fmt.Errorf("CreateWorkspace.ParseOrgID: %w", err)
	}
	if _, err := s.orgRepo.GetByID(ctx, orgID); err != nil {
		return nil, fmt.Errorf("CreateWorkspace.GetOrg: %w", err)
	}
	ws, err := tenant.NewWorkspace(orgID, in.Name, in.Slug, in.Description)
	if err != nil {
		return nil, fmt.Errorf("CreateWorkspace.NewWorkspace: %w", err)
	}
	if err := s.wsRepo.Create(ctx, ws); err != nil {
		return nil, fmt.Errorf("CreateWorkspace.Create: %w", err)
	}
	return ws, nil
}

func (s *TenantService) GetWorkspace(ctx context.Context, id string) (*tenant.Workspace, error) {
	uid, err := uuid.Parse(id)
	if err != nil {
		return nil, fmt.Errorf("GetWorkspace.ParseID: %w", err)
	}
	ws, err := s.wsRepo.GetByID(ctx, uid)
	if err != nil {
		return nil, fmt.Errorf("GetWorkspace.GetByID: %w", err)
	}
	return ws, nil
}

func (s *TenantService) ListWorkspaces(ctx context.Context, orgID string) ([]*tenant.Workspace, error) {
	uid, err := uuid.Parse(orgID)
	if err != nil {
		return nil, fmt.Errorf("ListWorkspaces.ParseOrgID: %w", err)
	}
	wss, err := s.wsRepo.ListByOrg(ctx, uid)
	if err != nil {
		return nil, fmt.Errorf("ListWorkspaces.ListByOrg: %w", err)
	}
	return wss, nil
}

type UpdateWorkspaceInput struct {
	Name        string
	Slug        string
	Description string
}

func (s *TenantService) UpdateWorkspace(ctx context.Context, id string, in UpdateWorkspaceInput) (*tenant.Workspace, error) {
	uid, err := uuid.Parse(id)
	if err != nil {
		return nil, fmt.Errorf("UpdateWorkspace.ParseID: %w", err)
	}
	ws, err := s.wsRepo.GetByID(ctx, uid)
	if err != nil {
		return nil, fmt.Errorf("UpdateWorkspace.GetByID: %w", err)
	}
	if in.Name != "" {
		ws.Name = in.Name
	}
	if in.Slug != "" {
		ws.Slug = in.Slug
	}
	if in.Description != "" {
		ws.Description = in.Description
	}
	ws.UpdatedAt = time.Now().UTC()
	if err := s.wsRepo.Update(ctx, ws); err != nil {
		return nil, fmt.Errorf("UpdateWorkspace.Update: %w", err)
	}
	return ws, nil
}

func (s *TenantService) DeleteWorkspace(ctx context.Context, id string) error {
	uid, err := uuid.Parse(id)
	if err != nil {
		return fmt.Errorf("DeleteWorkspace.ParseID: %w", err)
	}
	if err := s.wsRepo.Delete(ctx, uid); err != nil {
		return fmt.Errorf("DeleteWorkspace.Delete: %w", err)
	}
	return nil
}

// --- Workspace member management ---

func (s *TenantService) ListWorkspaceMembers(ctx context.Context, workspaceID string) ([]tenant.WorkspaceMember, error) {
	if _, err := uuid.Parse(workspaceID); err != nil {
		return nil, fmt.Errorf("ListWorkspaceMembers.ParseID: %w", err)
	}
	members, err := s.wsRepo.ListMembers(ctx, workspaceID)
	if err != nil {
		return nil, fmt.Errorf("ListWorkspaceMembers: %w", err)
	}
	return members, nil
}

func (s *TenantService) AddWorkspaceMember(ctx context.Context, workspaceID, userID, role string) error {
	if _, err := uuid.Parse(workspaceID); err != nil {
		return fmt.Errorf("AddWorkspaceMember.ParseWorkspaceID: %w", err)
	}
	if !tenant.ValidRoles[role] {
		return fmt.Errorf("AddWorkspaceMember: %w", tenant.ErrInvalidRole)
	}
	m := tenant.WorkspaceMember{
		ID:          uuid.New().String(),
		WorkspaceID: workspaceID,
		UserID:      userID,
		Role:        role,
		JoinedAt:    time.Now().UTC(),
	}
	if err := s.wsRepo.AddMember(ctx, m); err != nil {
		return fmt.Errorf("AddWorkspaceMember: %w", err)
	}
	return nil
}

func (s *TenantService) UpdateWorkspaceMember(ctx context.Context, workspaceID, userID, role string) error {
	if !tenant.ValidRoles[role] {
		return fmt.Errorf("UpdateWorkspaceMember: %w", tenant.ErrInvalidRole)
	}
	if err := s.wsRepo.UpdateMember(ctx, workspaceID, userID, role); err != nil {
		return fmt.Errorf("UpdateWorkspaceMember: %w", err)
	}
	return nil
}

func (s *TenantService) RemoveWorkspaceMember(ctx context.Context, workspaceID, userID string) error {
	if err := s.wsRepo.RemoveMember(ctx, workspaceID, userID); err != nil {
		return fmt.Errorf("RemoveWorkspaceMember: %w", err)
	}
	return nil
}

// --- API key management ---

func (s *TenantService) ListAPIKeys(ctx context.Context, tenantID string) ([]tenant.APIKey, error) {
	keys, err := s.apiKeyRepo.ListAPIKeys(ctx, tenantID)
	if err != nil {
		return nil, fmt.Errorf("ListAPIKeys: %w", err)
	}
	return keys, nil
}

// CreateAPIKey generates a new API key and returns the model and the plain-text key (shown only once).
func (s *TenantService) CreateAPIKey(ctx context.Context, tenantID, userID, name string, scopes []string) (tenant.APIKey, string, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return tenant.APIKey{}, "", fmt.Errorf("CreateAPIKey.rand: %w", err)
	}
	plainKey := hex.EncodeToString(raw) // 64-char hex string
	prefix := plainKey[:8]
	h := sha256.Sum256([]byte(plainKey))
	keyHash := hex.EncodeToString(h[:])

	key := tenant.APIKey{
		ID:        uuid.New().String(),
		TenantID:  tenantID,
		UserID:    userID,
		Name:      name,
		KeyPrefix: prefix,
		KeyHash:   keyHash,
		Scopes:    scopes,
		CreatedAt: time.Now().UTC(),
	}
	if err := s.apiKeyRepo.CreateAPIKey(ctx, key); err != nil {
		return tenant.APIKey{}, "", fmt.Errorf("CreateAPIKey: %w", err)
	}
	return key, plainKey, nil
}

func (s *TenantService) RevokeAPIKey(ctx context.Context, id, tenantID string) error {
	if err := s.apiKeyRepo.DeleteAPIKey(ctx, id, tenantID); err != nil {
		return fmt.Errorf("RevokeAPIKey: %w", err)
	}
	return nil
}
