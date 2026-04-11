package http

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/greenlab/iam/internal/domain/tenant"
	"github.com/stretchr/testify/assert"
)

func TestToWorkspaceResponse_MemberCount(t *testing.T) {
	now := time.Now().UTC()
	ws := &tenant.Workspace{
		ID:          uuid.New(),
		OrgID:       uuid.New(),
		Name:        "My Workspace",
		Slug:        "my-workspace",
		Description: "A test workspace",
		MemberCount: 5,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	resp := toWorkspaceResponse(ws)

	assert.Equal(t, ws.ID.String(), resp.ID)
	assert.Equal(t, ws.OrgID.String(), resp.OrgID)
	assert.Equal(t, ws.Name, resp.Name)
	assert.Equal(t, ws.Slug, resp.Slug)
	assert.Equal(t, ws.Description, resp.Description)
	assert.Equal(t, 5, resp.MemberCount)
	assert.Equal(t, now, resp.CreatedAt)
}

func TestToWorkspaceResponse_ZeroMemberCount(t *testing.T) {
	ws := &tenant.Workspace{
		ID:          uuid.New(),
		OrgID:       uuid.New(),
		Name:        "Empty Workspace",
		Slug:        "empty",
		MemberCount: 0,
		CreatedAt:   time.Now().UTC(),
	}

	resp := toWorkspaceResponse(ws)

	assert.Equal(t, 0, resp.MemberCount)
}
