package application

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/greenlab/iam/internal/domain/tenant"
	mocktenant "github.com/greenlab/iam/internal/mocks/tenant"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func newTestTenantService(t *testing.T) (
	*TenantService,
	*mocktenant.MockOrgRepository,
	*mocktenant.MockWorkspaceRepository,
	*mocktenant.MockAPIKeyRepository,
) {
	t.Helper()
	orgRepo := mocktenant.NewMockOrgRepository(t)
	wsRepo := mocktenant.NewMockWorkspaceRepository(t)
	apiKeyRepo := mocktenant.NewMockAPIKeyRepository(t)
	svc := NewTenantService(orgRepo, wsRepo, apiKeyRepo)
	return svc, orgRepo, wsRepo, apiKeyRepo
}

// --- Org ---

func TestCreateOrg_Success(t *testing.T) {
	svc, orgRepo, _, _ := newTestTenantService(t)
	ctx := context.Background()

	ownerID := uuid.New()
	orgRepo.On("GetBySlug", ctx, "my-org").Return(nil, tenant.ErrOrgNotFound)
	orgRepo.On("Create", ctx, mock.AnythingOfType("*tenant.Org")).Return(nil)

	org, err := svc.CreateOrg(ctx, CreateOrgInput{
		Name:        "My Org",
		Slug:        "my-org",
		OwnerUserID: ownerID.String(),
	})
	require.NoError(t, err)
	assert.NotNil(t, org)
	assert.Equal(t, "My Org", org.Name)
	assert.Equal(t, "my-org", org.Slug)
}

func TestCreateOrg_SlugAlreadyTaken(t *testing.T) {
	svc, orgRepo, _, _ := newTestTenantService(t)
	ctx := context.Background()

	existing := &tenant.Org{ID: uuid.New(), Slug: "taken-slug"}
	orgRepo.On("GetBySlug", ctx, "taken-slug").Return(existing, nil)

	org, err := svc.CreateOrg(ctx, CreateOrgInput{
		Name:        "Another Org",
		Slug:        "taken-slug",
		OwnerUserID: uuid.New().String(),
	})
	assert.Error(t, err)
	assert.Nil(t, org)
	assert.ErrorIs(t, err, tenant.ErrSlugAlreadyTaken)
}

func TestCreateOrg_InvalidOwnerUUID(t *testing.T) {
	svc, _, _, _ := newTestTenantService(t)
	ctx := context.Background()

	org, err := svc.CreateOrg(ctx, CreateOrgInput{
		Name:        "My Org",
		Slug:        "my-org",
		OwnerUserID: "not-a-uuid",
	})
	assert.Error(t, err)
	assert.Nil(t, org)
}

func TestGetOrg_Success(t *testing.T) {
	svc, orgRepo, _, _ := newTestTenantService(t)
	ctx := context.Background()

	orgID := uuid.New()
	expected := &tenant.Org{ID: orgID, Name: "Test Org", Slug: "test-org"}
	orgRepo.On("GetByID", ctx, orgID).Return(expected, nil)

	org, err := svc.GetOrg(ctx, orgID.String())
	require.NoError(t, err)
	assert.Equal(t, expected, org)
}

func TestGetOrg_NotFound(t *testing.T) {
	svc, orgRepo, _, _ := newTestTenantService(t)
	ctx := context.Background()

	orgID := uuid.New()
	orgRepo.On("GetByID", ctx, orgID).Return(nil, tenant.ErrOrgNotFound)

	org, err := svc.GetOrg(ctx, orgID.String())
	assert.Error(t, err)
	assert.Nil(t, org)
	assert.ErrorIs(t, err, tenant.ErrOrgNotFound)
}

func TestListOrgs_Success(t *testing.T) {
	svc, orgRepo, _, _ := newTestTenantService(t)
	ctx := context.Background()

	orgs := []*tenant.Org{
		{ID: uuid.New(), Name: "Org 1"},
		{ID: uuid.New(), Name: "Org 2"},
	}
	orgRepo.On("List", ctx, 10, 0).Return(orgs, int64(2), nil)

	result, total, err := svc.ListOrgs(ctx, 10, 0)
	require.NoError(t, err)
	assert.Equal(t, int64(2), total)
	assert.Len(t, result, 2)
}

func TestUpdateOrg_Success(t *testing.T) {
	svc, orgRepo, _, _ := newTestTenantService(t)
	ctx := context.Background()

	orgID := uuid.New()
	existing := &tenant.Org{ID: orgID, Name: "Old Name", Slug: "my-org"}
	orgRepo.On("GetByID", ctx, orgID).Return(existing, nil)
	orgRepo.On("Update", ctx, mock.AnythingOfType("*tenant.Org")).Return(nil)

	org, err := svc.UpdateOrg(ctx, orgID.String(), UpdateOrgInput{Name: "New Name"})
	require.NoError(t, err)
	assert.Equal(t, "New Name", org.Name)
}

func TestUpdateOrg_NotFound(t *testing.T) {
	svc, orgRepo, _, _ := newTestTenantService(t)
	ctx := context.Background()

	orgID := uuid.New()
	orgRepo.On("GetByID", ctx, orgID).Return(nil, tenant.ErrOrgNotFound)

	org, err := svc.UpdateOrg(ctx, orgID.String(), UpdateOrgInput{Name: "New Name"})
	assert.Error(t, err)
	assert.Nil(t, org)
	assert.ErrorIs(t, err, tenant.ErrOrgNotFound)
}

func TestDeleteOrg_Success(t *testing.T) {
	svc, orgRepo, _, _ := newTestTenantService(t)
	ctx := context.Background()

	orgID := uuid.New()
	orgRepo.On("Delete", ctx, orgID).Return(nil)

	err := svc.DeleteOrg(ctx, orgID.String())
	require.NoError(t, err)
}

func TestDeleteOrg_InvalidUUID(t *testing.T) {
	svc, _, _, _ := newTestTenantService(t)
	ctx := context.Background()

	err := svc.DeleteOrg(ctx, "not-a-uuid")
	assert.Error(t, err)
}

// --- Workspace ---

func TestCreateWorkspace_Success(t *testing.T) {
	svc, orgRepo, wsRepo, _ := newTestTenantService(t)
	ctx := context.Background()

	orgID := uuid.New()
	org := &tenant.Org{ID: orgID, Name: "My Org"}
	orgRepo.On("GetByID", ctx, orgID).Return(org, nil)
	wsRepo.On("Create", ctx, mock.AnythingOfType("*tenant.Workspace")).Return(nil)

	ws, err := svc.CreateWorkspace(ctx, CreateWorkspaceInput{
		OrgID:       orgID.String(),
		Name:        "My Workspace",
		Slug:        "my-workspace",
		Description: "A test workspace",
	})
	require.NoError(t, err)
	assert.NotNil(t, ws)
	assert.Equal(t, "My Workspace", ws.Name)
	assert.Equal(t, orgID, ws.OrgID)
}

func TestCreateWorkspace_OrgNotFound(t *testing.T) {
	svc, orgRepo, _, _ := newTestTenantService(t)
	ctx := context.Background()

	orgID := uuid.New()
	orgRepo.On("GetByID", ctx, orgID).Return(nil, tenant.ErrOrgNotFound)

	ws, err := svc.CreateWorkspace(ctx, CreateWorkspaceInput{
		OrgID:       orgID.String(),
		Name:        "My Workspace",
		Slug:        "my-workspace",
		Description: "A test workspace",
	})
	assert.Error(t, err)
	assert.Nil(t, ws)
	assert.ErrorIs(t, err, tenant.ErrOrgNotFound)
}

func TestGetWorkspace_Success(t *testing.T) {
	svc, _, wsRepo, _ := newTestTenantService(t)
	ctx := context.Background()

	wsID := uuid.New()
	expected := &tenant.Workspace{ID: wsID, Name: "My WS"}
	wsRepo.On("GetByID", ctx, wsID).Return(expected, nil)

	ws, err := svc.GetWorkspace(ctx, wsID.String())
	require.NoError(t, err)
	assert.Equal(t, expected, ws)
}

func TestGetWorkspace_NotFound(t *testing.T) {
	svc, _, wsRepo, _ := newTestTenantService(t)
	ctx := context.Background()

	wsID := uuid.New()
	wsRepo.On("GetByID", ctx, wsID).Return(nil, tenant.ErrWorkspaceNotFound)

	ws, err := svc.GetWorkspace(ctx, wsID.String())
	assert.Error(t, err)
	assert.Nil(t, ws)
}

func TestListWorkspaces_Success(t *testing.T) {
	svc, _, wsRepo, _ := newTestTenantService(t)
	ctx := context.Background()

	orgID := uuid.New()
	workspaces := []*tenant.Workspace{
		{ID: uuid.New(), Name: "WS 1"},
		{ID: uuid.New(), Name: "WS 2"},
	}
	wsRepo.On("ListByOrg", ctx, orgID).Return(workspaces, nil)

	result, err := svc.ListWorkspaces(ctx, orgID.String())
	require.NoError(t, err)
	assert.Len(t, result, 2)
}

func TestUpdateWorkspace_Success(t *testing.T) {
	svc, _, wsRepo, _ := newTestTenantService(t)
	ctx := context.Background()

	wsID := uuid.New()
	existing := &tenant.Workspace{ID: wsID, Name: "Old Name", Slug: "old-slug"}
	wsRepo.On("GetByID", ctx, wsID).Return(existing, nil)
	wsRepo.On("Update", ctx, mock.AnythingOfType("*tenant.Workspace")).Return(nil)

	ws, err := svc.UpdateWorkspace(ctx, wsID.String(), UpdateWorkspaceInput{Name: "New Name"})
	require.NoError(t, err)
	assert.Equal(t, "New Name", ws.Name)
}

func TestUpdateWorkspace_NotFound(t *testing.T) {
	svc, _, wsRepo, _ := newTestTenantService(t)
	ctx := context.Background()

	wsID := uuid.New()
	wsRepo.On("GetByID", ctx, wsID).Return(nil, tenant.ErrWorkspaceNotFound)

	ws, err := svc.UpdateWorkspace(ctx, wsID.String(), UpdateWorkspaceInput{Name: "New Name"})
	assert.Error(t, err)
	assert.Nil(t, ws)
}

func TestDeleteWorkspace_Success(t *testing.T) {
	svc, _, wsRepo, _ := newTestTenantService(t)
	ctx := context.Background()

	wsID := uuid.New()
	wsRepo.On("Delete", ctx, wsID).Return(nil)

	err := svc.DeleteWorkspace(ctx, wsID.String())
	require.NoError(t, err)
}

// --- Workspace members ---

func TestListWorkspaceMembers_Success(t *testing.T) {
	svc, _, wsRepo, _ := newTestTenantService(t)
	ctx := context.Background()

	wsID := uuid.New()
	members := []tenant.WorkspaceMember{
		{ID: uuid.New().String(), WorkspaceID: wsID.String(), UserID: uuid.New().String(), Role: "viewer"},
	}
	wsRepo.On("ListMembers", ctx, wsID.String()).Return(members, nil)

	result, err := svc.ListWorkspaceMembers(ctx, wsID.String())
	require.NoError(t, err)
	assert.Len(t, result, 1)
}

func TestAddWorkspaceMember_Success(t *testing.T) {
	svc, _, wsRepo, _ := newTestTenantService(t)
	ctx := context.Background()

	wsID := uuid.New()
	userID := uuid.New().String()
	wsRepo.On("AddMember", ctx, mock.AnythingOfType("tenant.WorkspaceMember")).Return(nil)

	err := svc.AddWorkspaceMember(ctx, wsID.String(), userID, "member")
	require.NoError(t, err)
}

func TestAddWorkspaceMember_InvalidRole(t *testing.T) {
	svc, _, _, _ := newTestTenantService(t)
	ctx := context.Background()

	err := svc.AddWorkspaceMember(ctx, uuid.New().String(), uuid.New().String(), "superuser")
	assert.Error(t, err)
	assert.ErrorIs(t, err, tenant.ErrInvalidRole)
}

func TestUpdateWorkspaceMember_Success(t *testing.T) {
	svc, _, wsRepo, _ := newTestTenantService(t)
	ctx := context.Background()

	wsID := uuid.New().String()
	userID := uuid.New().String()
	wsRepo.On("UpdateMember", ctx, wsID, userID, "admin").Return(nil)

	err := svc.UpdateWorkspaceMember(ctx, wsID, userID, "admin")
	require.NoError(t, err)
}

func TestUpdateWorkspaceMember_InvalidRole(t *testing.T) {
	svc, _, _, _ := newTestTenantService(t)
	ctx := context.Background()

	err := svc.UpdateWorkspaceMember(ctx, uuid.New().String(), uuid.New().String(), "invalid")
	assert.Error(t, err)
	assert.ErrorIs(t, err, tenant.ErrInvalidRole)
}

func TestRemoveWorkspaceMember_Success(t *testing.T) {
	svc, _, wsRepo, _ := newTestTenantService(t)
	ctx := context.Background()

	wsID := uuid.New().String()
	userID := uuid.New().String()
	wsRepo.On("RemoveMember", ctx, wsID, userID).Return(nil)

	err := svc.RemoveWorkspaceMember(ctx, wsID, userID)
	require.NoError(t, err)
}

// --- API keys ---

func TestListAPIKeys_Success(t *testing.T) {
	svc, _, _, apiKeyRepo := newTestTenantService(t)
	ctx := context.Background()

	tenantID := uuid.New().String()
	keys := []tenant.APIKey{
		{ID: uuid.New().String(), TenantID: tenantID, Name: "Key 1"},
	}
	apiKeyRepo.On("ListAPIKeys", ctx, tenantID).Return(keys, nil)

	result, err := svc.ListAPIKeys(ctx, tenantID)
	require.NoError(t, err)
	assert.Len(t, result, 1)
}

func TestCreateAPIKey_Success(t *testing.T) {
	svc, _, _, apiKeyRepo := newTestTenantService(t)
	ctx := context.Background()

	tenantID := uuid.New().String()
	userID := uuid.New().String()
	apiKeyRepo.On("CreateAPIKey", ctx, mock.AnythingOfType("tenant.APIKey")).Return(nil)

	key, plainKey, err := svc.CreateAPIKey(ctx, tenantID, userID, "My Key", []string{"read"})
	require.NoError(t, err)
	assert.Len(t, plainKey, 64)
	assert.Equal(t, plainKey[:8], key.KeyPrefix)
	assert.Equal(t, tenantID, key.TenantID)
	assert.Equal(t, "My Key", key.Name)
}

func TestRevokeAPIKey_Success(t *testing.T) {
	svc, _, _, apiKeyRepo := newTestTenantService(t)
	ctx := context.Background()

	keyID := uuid.New().String()
	tenantID := uuid.New().String()
	apiKeyRepo.On("DeleteAPIKey", ctx, keyID, tenantID).Return(nil)

	err := svc.RevokeAPIKey(ctx, keyID, tenantID)
	require.NoError(t, err)
}
