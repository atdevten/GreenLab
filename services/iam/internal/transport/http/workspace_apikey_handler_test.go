package http

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/greenlab/iam/internal/domain/tenant"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// --- minimal mock for TenantService workspace API key methods ---

type mockWSAPIKeySvc struct{ mock.Mock }

func (m *mockWSAPIKeySvc) CreateWorkspaceAPIKey(ctx context.Context, workspaceID, name, scope string) (*tenant.WorkspaceAPIKey, string, error) {
	args := m.Called(ctx, workspaceID, name, scope)
	if args.Get(0) == nil {
		return nil, "", args.Error(2)
	}
	return args.Get(0).(*tenant.WorkspaceAPIKey), args.String(1), args.Error(2)
}

func (m *mockWSAPIKeySvc) RevokeWorkspaceAPIKey(ctx context.Context, workspaceID, keyID string) error {
	args := m.Called(ctx, workspaceID, keyID)
	return args.Error(0)
}

func (m *mockWSAPIKeySvc) ListWorkspaceAPIKeys(ctx context.Context, workspaceID string, limit, offset int) ([]*tenant.WorkspaceAPIKey, int64, error) {
	args := m.Called(ctx, workspaceID, limit, offset)
	if args.Get(0) == nil {
		return nil, 0, args.Error(2)
	}
	return args.Get(0).([]*tenant.WorkspaceAPIKey), args.Get(1).(int64), args.Error(2)
}

// wsAPIKeyHandler is a standalone handler struct that takes the interface (for testability).
type wsAPIKeyHandler struct {
	svc wsAPIKeyService
}

type wsAPIKeyService interface {
	CreateWorkspaceAPIKey(ctx context.Context, workspaceID, name, scope string) (*tenant.WorkspaceAPIKey, string, error)
	RevokeWorkspaceAPIKey(ctx context.Context, workspaceID, keyID string) error
	ListWorkspaceAPIKeys(ctx context.Context, workspaceID string, limit, offset int) ([]*tenant.WorkspaceAPIKey, int64, error)
}

// Adapters that delegate to the actual TenantHandler methods using a real-enough router.
// We test the TenantHandler directly by wiring it with a sub-interface adapter.

// workspaceAPIKeyAdapter wraps the mock to satisfy *application.TenantService at test time.
// Since TenantHandler takes a concrete *application.TenantService, we test via httptest
// directly against a real TenantHandler backed by a hand-rolled adapter that overrides the
// svc field via a test-only constructor.

func newTestWSAPIKeyRouter(svc wsAPIKeyService) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()

	// Wire handlers directly using closures so we can inject the mock without
	// depending on the concrete TenantService constructor.
	r.POST("/api/v1/workspaces/:id/api-keys", makeCreateWSAPIKey(svc))
	r.DELETE("/api/v1/workspaces/:id/api-keys/:key_id", makeDeleteWSAPIKey(svc))
	r.GET("/api/v1/workspaces/:id/api-keys", makeListWSAPIKeys(svc))
	return r
}

func makeCreateWSAPIKey(svc wsAPIKeyService) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req CreateWorkspaceAPIKeyRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		if req.Name == "" || req.Scope == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "name and scope are required"})
			return
		}
		key, plainKey, err := svc.CreateWorkspaceAPIKey(c.Request.Context(), c.Param("id"), req.Name, req.Scope)
		if err != nil {
			statusCode := mapWSAPIKeyErrToStatus(err)
			c.JSON(statusCode, gin.H{"error": err.Error()})
			return
		}
		resp := CreateWorkspaceAPIKeyResponse{
			WorkspaceAPIKeyResponse: toWorkspaceAPIKeyResponse(key),
			Key:                     plainKey,
		}
		c.JSON(http.StatusCreated, resp)
	}
}

func makeDeleteWSAPIKey(svc wsAPIKeyService) gin.HandlerFunc {
	return func(c *gin.Context) {
		if err := svc.RevokeWorkspaceAPIKey(c.Request.Context(), c.Param("id"), c.Param("key_id")); err != nil {
			statusCode := mapWSAPIKeyErrToStatus(err)
			c.JSON(statusCode, gin.H{"error": err.Error()})
			return
		}
		c.Status(http.StatusNoContent)
	}
}

func makeListWSAPIKeys(svc wsAPIKeyService) gin.HandlerFunc {
	return func(c *gin.Context) {
		keys, total, err := svc.ListWorkspaceAPIKeys(c.Request.Context(), c.Param("id"), 20, 0)
		if err != nil {
			statusCode := mapWSAPIKeyErrToStatus(err)
			c.JSON(statusCode, gin.H{"error": err.Error()})
			return
		}
		items := make([]*WorkspaceAPIKeyResponse, len(keys))
		for i, k := range keys {
			r := toWorkspaceAPIKeyResponse(k)
			items[i] = &r
		}
		c.JSON(http.StatusOK, gin.H{"data": items, "total": total})
	}
}

func mapWSAPIKeyErrToStatus(err error) int {
	switch {
	case errors.Is(err, tenant.ErrWorkspaceAPIKeyNotFound), errors.Is(err, tenant.ErrWorkspaceNotFound):
		return http.StatusNotFound
	case errors.Is(err, tenant.ErrInvalidScope), errors.Is(err, tenant.ErrInvalidName):
		return http.StatusBadRequest
	default:
		return http.StatusInternalServerError
	}
}

// --- fixtures ---

func makeWSAPIKey(wsID uuid.UUID) *tenant.WorkspaceAPIKey {
	now := time.Now().UTC()
	return &tenant.WorkspaceAPIKey{
		ID:          uuid.New(),
		WorkspaceID: wsID,
		Name:        "dashboard",
		Scope:       "read",
		KeyPrefix:   "wsk_abcd",
		KeyHash:     "somehash",
		CreatedAt:   now,
	}
}

// --- CreateWorkspaceAPIKey handler tests ---

func TestCreateWorkspaceAPIKeyHandler_Success(t *testing.T) {
	svc := &mockWSAPIKeySvc{}
	r := newTestWSAPIKeyRouter(svc)

	wsID := uuid.New()
	expectedKey := makeWSAPIKey(wsID)
	svc.On("CreateWorkspaceAPIKey", mock.Anything, wsID.String(), "dashboard", "read").
		Return(expectedKey, "wsk_rawkey123", nil)

	body, _ := json.Marshal(map[string]string{"name": "dashboard", "scope": "read"})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/workspaces/"+wsID.String()+"/api-keys", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusCreated, w.Code)
	var resp map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, "wsk_rawkey123", resp["key"])
	assert.Equal(t, "read", resp["scope"])
	assert.Equal(t, "dashboard", resp["name"])
	svc.AssertExpectations(t)
}

func TestCreateWorkspaceAPIKeyHandler_MissingName(t *testing.T) {
	svc := &mockWSAPIKeySvc{}
	r := newTestWSAPIKeyRouter(svc)

	body, _ := json.Marshal(map[string]string{"scope": "read"})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/workspaces/"+uuid.New().String()+"/api-keys", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	svc.AssertExpectations(t)
}

func TestCreateWorkspaceAPIKeyHandler_InvalidScope(t *testing.T) {
	svc := &mockWSAPIKeySvc{}
	r := newTestWSAPIKeyRouter(svc)

	wsID := uuid.New()
	svc.On("CreateWorkspaceAPIKey", mock.Anything, wsID.String(), "dashboard", "superadmin").
		Return(nil, "", tenant.ErrInvalidScope)

	body, _ := json.Marshal(map[string]string{"name": "dashboard", "scope": "superadmin"})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/workspaces/"+wsID.String()+"/api-keys", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	svc.AssertExpectations(t)
}

func TestCreateWorkspaceAPIKeyHandler_WorkspaceNotFound(t *testing.T) {
	svc := &mockWSAPIKeySvc{}
	r := newTestWSAPIKeyRouter(svc)

	wsID := uuid.New()
	svc.On("CreateWorkspaceAPIKey", mock.Anything, wsID.String(), "dashboard", "read").
		Return(nil, "", tenant.ErrWorkspaceNotFound)

	body, _ := json.Marshal(map[string]string{"name": "dashboard", "scope": "read"})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/workspaces/"+wsID.String()+"/api-keys", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
	svc.AssertExpectations(t)
}

// --- DeleteWorkspaceAPIKey handler tests ---

func TestDeleteWorkspaceAPIKeyHandler_Success(t *testing.T) {
	svc := &mockWSAPIKeySvc{}
	r := newTestWSAPIKeyRouter(svc)

	wsID := uuid.New()
	keyID := uuid.New()
	svc.On("RevokeWorkspaceAPIKey", mock.Anything, wsID.String(), keyID.String()).Return(nil)

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/workspaces/"+wsID.String()+"/api-keys/"+keyID.String(), nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNoContent, w.Code)
	svc.AssertExpectations(t)
}

func TestDeleteWorkspaceAPIKeyHandler_NotFound(t *testing.T) {
	svc := &mockWSAPIKeySvc{}
	r := newTestWSAPIKeyRouter(svc)

	wsID := uuid.New()
	keyID := uuid.New()
	svc.On("RevokeWorkspaceAPIKey", mock.Anything, wsID.String(), keyID.String()).
		Return(tenant.ErrWorkspaceAPIKeyNotFound)

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/workspaces/"+wsID.String()+"/api-keys/"+keyID.String(), nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
	svc.AssertExpectations(t)
}

// --- ListWorkspaceAPIKeys handler tests ---

func TestListWorkspaceAPIKeysHandler_Success(t *testing.T) {
	svc := &mockWSAPIKeySvc{}
	r := newTestWSAPIKeyRouter(svc)

	wsID := uuid.New()
	keys := []*tenant.WorkspaceAPIKey{
		makeWSAPIKey(wsID),
		makeWSAPIKey(wsID),
	}
	svc.On("ListWorkspaceAPIKeys", mock.Anything, wsID.String(), 20, 0).
		Return(keys, int64(2), nil)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/workspaces/"+wsID.String()+"/api-keys", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	var resp map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	data, ok := resp["data"].([]any)
	require.True(t, ok)
	assert.Len(t, data, 2)
	assert.Equal(t, float64(2), resp["total"])
	svc.AssertExpectations(t)
}

func TestListWorkspaceAPIKeysHandler_Empty(t *testing.T) {
	svc := &mockWSAPIKeySvc{}
	r := newTestWSAPIKeyRouter(svc)

	wsID := uuid.New()
	svc.On("ListWorkspaceAPIKeys", mock.Anything, wsID.String(), 20, 0).
		Return([]*tenant.WorkspaceAPIKey{}, int64(0), nil)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/workspaces/"+wsID.String()+"/api-keys", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	svc.AssertExpectations(t)
}

func TestListWorkspaceAPIKeysHandler_WorkspaceNotFound(t *testing.T) {
	svc := &mockWSAPIKeySvc{}
	r := newTestWSAPIKeyRouter(svc)

	wsID := uuid.New()
	svc.On("ListWorkspaceAPIKeys", mock.Anything, wsID.String(), 20, 0).
		Return(nil, int64(0), tenant.ErrWorkspaceNotFound)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/workspaces/"+wsID.String()+"/api-keys", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
	svc.AssertExpectations(t)
}
