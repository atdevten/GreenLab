package http

import (
	"errors"

	"github.com/gin-gonic/gin"
	"github.com/greenlab/iam/internal/application"
	"github.com/greenlab/iam/internal/domain/tenant"
	"github.com/greenlab/shared/pkg/apierr"
	"github.com/greenlab/shared/pkg/middleware"
	"github.com/greenlab/shared/pkg/pagination"
	"github.com/greenlab/shared/pkg/response"
	"github.com/greenlab/shared/pkg/validator"
)

type TenantHandler struct {
	svc *application.TenantService
}

func NewTenantHandler(svc *application.TenantService) *TenantHandler {
	return &TenantHandler{svc: svc}
}

// CreateOrg godoc
// @Summary      Create a new organisation
// @Tags         orgs
// @Accept       json
// @Produce      json
// @Param        request  body      CreateOrgRequest  true  "Organisation details"
// @Success      201      {object}  OrgResponse
// @Failure      400      {object}  map[string]interface{}
// @Failure      409      {object}  map[string]interface{}
// @Security     BearerAuth
// @Router       /api/v1/orgs [post]
func (h *TenantHandler) CreateOrg(c *gin.Context) {
	var req CreateOrgRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, apierr.BadRequest(err.Error()))
		return
	}
	if err := validator.Validate(&req); err != nil {
		response.ValidationError(c, err)
		return
	}
	org, err := h.svc.CreateOrg(c.Request.Context(), application.CreateOrgInput{
		Name: req.Name, Slug: req.Slug, OwnerUserID: req.OwnerUserID,
	})
	if err != nil {
		response.Error(c, mapTenantError(err))
		return
	}
	response.Created(c, toOrgResponse(org))
}

// GetOrg godoc
// @Summary      Get an organisation by ID
// @Tags         orgs
// @Produce      json
// @Param        id  path      string  true  "Organisation ID"
// @Success      200  {object}  OrgResponse
// @Failure      404  {object}  map[string]interface{}
// @Security     BearerAuth
// @Router       /api/v1/orgs/{id} [get]
func (h *TenantHandler) GetOrg(c *gin.Context) {
	org, err := h.svc.GetOrg(c.Request.Context(), c.Param("id"))
	if err != nil {
		response.Error(c, mapTenantError(err))
		return
	}
	response.OK(c, toOrgResponse(org))
}

// ListOrgs godoc
// @Summary      List all organisations
// @Tags         orgs
// @Produce      json
// @Param        limit   query  int  false  "Page size"
// @Param        offset  query  int  false  "Page offset"
// @Success      200  {array}   OrgResponse
// @Failure      500  {object}  map[string]interface{}
// @Security     BearerAuth
// @Router       /api/v1/orgs [get]
func (h *TenantHandler) ListOrgs(c *gin.Context) {
	page := pagination.ParseOffset(c)
	orgs, total, err := h.svc.ListOrgs(c.Request.Context(), page.Limit, page.Offset())
	if err != nil {
		response.Error(c, apierr.Internal(err))
		return
	}
	items := make([]*OrgResponse, len(orgs))
	for i, o := range orgs {
		items[i] = toOrgResponse(o)
	}
	response.OKWithMeta(c, items, pagination.NewOffsetResult(items, total, page))
}

// UpdateOrg godoc
// @Summary      Update an organisation
// @Tags         orgs
// @Accept       json
// @Produce      json
// @Param        id       path      string           true  "Organisation ID"
// @Param        request  body      UpdateOrgRequest  true  "Update fields"
// @Success      200      {object}  OrgResponse
// @Failure      400      {object}  map[string]interface{}
// @Failure      404      {object}  map[string]interface{}
// @Security     BearerAuth
// @Router       /api/v1/orgs/{id} [put]
func (h *TenantHandler) UpdateOrg(c *gin.Context) {
	var req UpdateOrgRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, apierr.BadRequest(err.Error()))
		return
	}
	if err := validator.Validate(&req); err != nil {
		response.ValidationError(c, err)
		return
	}
	org, err := h.svc.UpdateOrg(c.Request.Context(), c.Param("id"), application.UpdateOrgInput{
		Name: req.Name, LogoURL: req.LogoURL, Website: req.Website,
	})
	if err != nil {
		response.Error(c, mapTenantError(err))
		return
	}
	response.OK(c, toOrgResponse(org))
}

// DeleteOrg godoc
// @Summary      Delete an organisation
// @Tags         orgs
// @Param        id  path  string  true  "Organisation ID"
// @Success      204  "No Content"
// @Failure      404  {object}  map[string]interface{}
// @Security     BearerAuth
// @Router       /api/v1/orgs/{id} [delete]
func (h *TenantHandler) DeleteOrg(c *gin.Context) {
	if err := h.svc.DeleteOrg(c.Request.Context(), c.Param("id")); err != nil {
		response.Error(c, mapTenantError(err))
		return
	}
	response.NoContent(c)
}

// CreateWorkspace godoc
// @Summary      Create a new workspace
// @Tags         workspaces
// @Accept       json
// @Produce      json
// @Param        request  body      CreateWorkspaceRequest  true  "Workspace details"
// @Success      201      {object}  WorkspaceResponse
// @Failure      400      {object}  map[string]interface{}
// @Failure      409      {object}  map[string]interface{}
// @Security     BearerAuth
// @Router       /api/v1/workspaces [post]
func (h *TenantHandler) CreateWorkspace(c *gin.Context) {
	var req CreateWorkspaceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, apierr.BadRequest(err.Error()))
		return
	}
	if err := validator.Validate(&req); err != nil {
		response.ValidationError(c, err)
		return
	}
	ws, err := h.svc.CreateWorkspace(c.Request.Context(), application.CreateWorkspaceInput{
		OrgID: req.OrgID, Name: req.Name, Slug: req.Slug, Description: req.Description,
	})
	if err != nil {
		response.Error(c, mapTenantError(err))
		return
	}
	response.Created(c, toWorkspaceResponse(ws))
}

// ListWorkspaces godoc
// @Summary      List workspaces for an organisation
// @Tags         workspaces
// @Produce      json
// @Param        id  path  string  true  "Organisation ID"
// @Success      200   {array}   WorkspaceResponse
// @Failure      404   {object}  map[string]interface{}
// @Security     BearerAuth
// @Router       /api/v1/orgs/{id}/workspaces [get]
func (h *TenantHandler) ListWorkspaces(c *gin.Context) {
	wss, err := h.svc.ListWorkspaces(c.Request.Context(), c.Param("id"))
	if err != nil {
		response.Error(c, mapTenantError(err))
		return
	}
	items := make([]*WorkspaceResponse, len(wss))
	for i, w := range wss {
		items[i] = toWorkspaceResponse(w)
	}
	response.OK(c, items)
}

// UpdateWorkspace godoc
// @Summary      Update a workspace
// @Tags         workspaces
// @Accept       json
// @Produce      json
// @Param        id       path      string                 true  "Workspace ID"
// @Param        request  body      UpdateWorkspaceRequest  true  "Update fields"
// @Success      200      {object}  WorkspaceResponse
// @Failure      400      {object}  map[string]interface{}
// @Failure      404      {object}  map[string]interface{}
// @Security     BearerAuth
// @Router       /api/v1/workspaces/{id} [put]
func (h *TenantHandler) UpdateWorkspace(c *gin.Context) {
	var req UpdateWorkspaceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, apierr.BadRequest(err.Error()))
		return
	}
	ws, err := h.svc.UpdateWorkspace(c.Request.Context(), c.Param("id"), application.UpdateWorkspaceInput{
		Name: req.Name, Slug: req.Slug, Description: req.Description,
	})
	if err != nil {
		response.Error(c, mapTenantError(err))
		return
	}
	response.OK(c, toWorkspaceResponse(ws))
}

// DeleteWorkspace godoc
// @Summary      Delete a workspace
// @Tags         workspaces
// @Param        id  path  string  true  "Workspace ID"
// @Success      204  "No Content"
// @Failure      404  {object}  map[string]interface{}
// @Security     BearerAuth
// @Router       /api/v1/workspaces/{id} [delete]
func (h *TenantHandler) DeleteWorkspace(c *gin.Context) {
	if err := h.svc.DeleteWorkspace(c.Request.Context(), c.Param("id")); err != nil {
		response.Error(c, mapTenantError(err))
		return
	}
	response.NoContent(c)
}

// ListMembers godoc
// @Summary      List members of a workspace
// @Tags         workspaces
// @Produce      json
// @Param        id  path      string  true  "Workspace ID"
// @Success      200  {array}   WorkspaceMemberResponse
// @Failure      404  {object}  map[string]interface{}
// @Security     BearerAuth
// @Router       /api/v1/workspaces/{id}/members [get]
func (h *TenantHandler) ListMembers(c *gin.Context) {
	members, err := h.svc.ListWorkspaceMembers(c.Request.Context(), c.Param("id"))
	if err != nil {
		response.Error(c, mapTenantError(err))
		return
	}
	items := make([]WorkspaceMemberResponse, len(members))
	for i, m := range members {
		items[i] = toMemberResponse(m)
	}
	response.OK(c, items)
}

// AddMember godoc
// @Summary      Add a member to a workspace
// @Tags         workspaces
// @Accept       json
// @Produce      json
// @Param        id       path      string           true  "Workspace ID"
// @Param        request  body      AddMemberRequest  true  "Member details"
// @Success      201      {object}  WorkspaceMemberResponse
// @Failure      400      {object}  map[string]interface{}
// @Failure      409      {object}  map[string]interface{}
// @Security     BearerAuth
// @Router       /api/v1/workspaces/{id}/members [post]
func (h *TenantHandler) AddMember(c *gin.Context) {
	var req AddMemberRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, apierr.BadRequest(err.Error()))
		return
	}
	if err := validator.Validate(&req); err != nil {
		response.ValidationError(c, err)
		return
	}
	if err := h.svc.AddWorkspaceMember(c.Request.Context(), c.Param("id"), req.UserID, req.Role); err != nil {
		response.Error(c, mapTenantError(err))
		return
	}
	response.Created(c, gin.H{"workspace_id": c.Param("id"), "user_id": req.UserID, "role": req.Role})
}

// UpdateMember godoc
// @Summary      Update a workspace member's role
// @Tags         workspaces
// @Accept       json
// @Produce      json
// @Param        id      path      string              true  "Workspace ID"
// @Param        userId  path      string              true  "User ID"
// @Param        request body      UpdateMemberRequest  true  "Role update"
// @Success      200     {object}  map[string]interface{}
// @Failure      400     {object}  map[string]interface{}
// @Failure      404     {object}  map[string]interface{}
// @Security     BearerAuth
// @Router       /api/v1/workspaces/{id}/members/{userId} [put]
func (h *TenantHandler) UpdateMember(c *gin.Context) {
	var req UpdateMemberRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, apierr.BadRequest(err.Error()))
		return
	}
	if err := validator.Validate(&req); err != nil {
		response.ValidationError(c, err)
		return
	}
	if err := h.svc.UpdateWorkspaceMember(c.Request.Context(), c.Param("id"), c.Param("userId"), req.Role); err != nil {
		response.Error(c, mapTenantError(err))
		return
	}
	response.OK(c, gin.H{"workspace_id": c.Param("id"), "user_id": c.Param("userId"), "role": req.Role})
}

// RemoveMember godoc
// @Summary      Remove a member from a workspace
// @Tags         workspaces
// @Param        id      path  string  true  "Workspace ID"
// @Param        userId  path  string  true  "User ID"
// @Success      204  "No Content"
// @Failure      404  {object}  map[string]interface{}
// @Security     BearerAuth
// @Router       /api/v1/workspaces/{id}/members/{userId} [delete]
func (h *TenantHandler) RemoveMember(c *gin.Context) {
	if err := h.svc.RemoveWorkspaceMember(c.Request.Context(), c.Param("id"), c.Param("userId")); err != nil {
		response.Error(c, mapTenantError(err))
		return
	}
	response.NoContent(c)
}

// ListAPIKeys godoc
// @Summary      List API keys for the authenticated tenant
// @Tags         api-keys
// @Produce      json
// @Success      200  {array}   APIKeyResponse
// @Failure      500  {object}  map[string]interface{}
// @Security     BearerAuth
// @Router       /api/v1/api-keys [get]
func (h *TenantHandler) ListAPIKeys(c *gin.Context) {
	tenantID, ok := middleware.GetTenantID(c)
	if !ok {
		response.Error(c, apierr.Unauthorized("missing tenant"))
		return
	}
	keys, err := h.svc.ListAPIKeys(c.Request.Context(), tenantID)
	if err != nil {
		response.Error(c, apierr.Internal(err))
		return
	}
	items := make([]APIKeyResponse, len(keys))
	for i, k := range keys {
		items[i] = toAPIKeyResponse(k)
	}
	response.OK(c, items)
}

// CreateAPIKey godoc
// @Summary      Create a new API key
// @Tags         api-keys
// @Accept       json
// @Produce      json
// @Param        request  body      CreateAPIKeyRequest    true  "API key details"
// @Success      201      {object}  CreateAPIKeyResponse
// @Failure      400      {object}  map[string]interface{}
// @Security     BearerAuth
// @Router       /api/v1/api-keys [post]
func (h *TenantHandler) CreateAPIKey(c *gin.Context) {
	tenantID, ok := middleware.GetTenantID(c)
	if !ok {
		response.Error(c, apierr.Unauthorized("missing tenant"))
		return
	}
	userID, ok := middleware.GetUserID(c)
	if !ok {
		response.Error(c, apierr.Unauthorized("missing user"))
		return
	}
	var req CreateAPIKeyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, apierr.BadRequest(err.Error()))
		return
	}
	if err := validator.Validate(&req); err != nil {
		response.ValidationError(c, err)
		return
	}
	key, plainKey, err := h.svc.CreateAPIKey(c.Request.Context(), tenantID, userID, req.Name, req.Scopes)
	if err != nil {
		response.Error(c, apierr.Internal(err))
		return
	}
	resp := CreateAPIKeyResponse{
		APIKeyResponse: toAPIKeyResponse(key),
		Key:            plainKey,
	}
	response.Created(c, resp)
}

// DeleteAPIKey godoc
// @Summary      Revoke an API key
// @Tags         api-keys
// @Param        id  path  string  true  "API Key ID"
// @Success      204  "No Content"
// @Failure      404  {object}  map[string]interface{}
// @Security     BearerAuth
// @Router       /api/v1/api-keys/{id} [delete]
func (h *TenantHandler) DeleteAPIKey(c *gin.Context) {
	tenantID, ok := middleware.GetTenantID(c)
	if !ok {
		response.Error(c, apierr.Unauthorized("missing tenant"))
		return
	}
	if err := h.svc.RevokeAPIKey(c.Request.Context(), c.Param("id"), tenantID); err != nil {
		response.Error(c, mapTenantError(err))
		return
	}
	response.NoContent(c)
}

// CreateWorkspaceAPIKey godoc
// @Summary      Create a scoped API key for a workspace
// @Tags         workspace-api-keys
// @Accept       json
// @Produce      json
// @Param        id       path      string                        true  "Workspace ID"
// @Param        request  body      CreateWorkspaceAPIKeyRequest  true  "API key details"
// @Success      201      {object}  CreateWorkspaceAPIKeyResponse
// @Failure      400      {object}  map[string]interface{}
// @Failure      404      {object}  map[string]interface{}
// @Security     BearerAuth
// @Router       /api/v1/workspaces/{id}/api-keys [post]
func (h *TenantHandler) CreateWorkspaceAPIKey(c *gin.Context) {
	var req CreateWorkspaceAPIKeyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, apierr.BadRequest(err.Error()))
		return
	}
	if err := validator.Validate(&req); err != nil {
		response.ValidationError(c, err)
		return
	}
	key, plainKey, err := h.svc.CreateWorkspaceAPIKey(c.Request.Context(), c.Param("id"), req.Name, req.Scope)
	if err != nil {
		response.Error(c, mapTenantError(err))
		return
	}
	resp := CreateWorkspaceAPIKeyResponse{
		WorkspaceAPIKeyResponse: toWorkspaceAPIKeyResponse(key),
		Key:                     plainKey,
	}
	response.Created(c, resp)
}

// DeleteWorkspaceAPIKey godoc
// @Summary      Revoke a workspace API key
// @Tags         workspace-api-keys
// @Param        id      path  string  true  "Workspace ID"
// @Param        key_id  path  string  true  "API Key ID"
// @Success      204  "No Content"
// @Failure      404  {object}  map[string]interface{}
// @Security     BearerAuth
// @Router       /api/v1/workspaces/{id}/api-keys/{key_id} [delete]
func (h *TenantHandler) DeleteWorkspaceAPIKey(c *gin.Context) {
	if err := h.svc.RevokeWorkspaceAPIKey(c.Request.Context(), c.Param("id"), c.Param("key_id")); err != nil {
		response.Error(c, mapTenantError(err))
		return
	}
	response.NoContent(c)
}

// ListWorkspaceAPIKeys godoc
// @Summary      List API keys for a workspace
// @Tags         workspace-api-keys
// @Produce      json
// @Param        id      path   string  true   "Workspace ID"
// @Param        limit   query  int     false  "Page size"
// @Param        offset  query  int     false  "Page offset"
// @Success      200  {array}   WorkspaceAPIKeyResponse
// @Failure      404  {object}  map[string]interface{}
// @Security     BearerAuth
// @Router       /api/v1/workspaces/{id}/api-keys [get]
func (h *TenantHandler) ListWorkspaceAPIKeys(c *gin.Context) {
	page := pagination.ParseOffset(c)
	keys, total, err := h.svc.ListWorkspaceAPIKeys(c.Request.Context(), c.Param("id"), page.Limit, page.Offset())
	if err != nil {
		response.Error(c, mapTenantError(err))
		return
	}
	items := make([]*WorkspaceAPIKeyResponse, len(keys))
	for i, k := range keys {
		r := toWorkspaceAPIKeyResponse(k)
		items[i] = &r
	}
	response.OKWithMeta(c, items, pagination.NewOffsetResult(items, total, page))
}

func toWorkspaceAPIKeyResponse(k *tenant.WorkspaceAPIKey) WorkspaceAPIKeyResponse {
	return WorkspaceAPIKeyResponse{
		ID:          k.ID.String(),
		WorkspaceID: k.WorkspaceID.String(),
		Name:        k.Name,
		Scope:       k.Scope,
		KeyPrefix:   k.KeyPrefix,
		CreatedAt:   k.CreatedAt,
		LastUsedAt:  k.LastUsedAt,
	}
}

func toOrgResponse(o *tenant.Org) *OrgResponse {
	return &OrgResponse{
		ID: o.ID.String(), Name: o.Name, Slug: o.Slug, Plan: string(o.Plan),
		OwnerUserID: o.OwnerUserID.String(), LogoURL: o.LogoURL, Website: o.Website,
		CreatedAt: o.CreatedAt,
	}
}

func toWorkspaceResponse(w *tenant.Workspace) *WorkspaceResponse {
	return &WorkspaceResponse{
		ID: w.ID.String(), OrgID: w.OrgID.String(), Name: w.Name,
		Slug: w.Slug, Description: w.Description, MemberCount: w.MemberCount,
		CreatedAt: w.CreatedAt,
	}
}

func toMemberResponse(m tenant.WorkspaceMember) WorkspaceMemberResponse {
	return WorkspaceMemberResponse{
		ID:          m.ID,
		WorkspaceID: m.WorkspaceID,
		UserID:      m.UserID,
		Name:        m.Name,
		Email:       m.Email,
		Role:        m.Role,
		JoinedAt:    m.JoinedAt,
	}
}

func toAPIKeyResponse(k tenant.APIKey) APIKeyResponse {
	return APIKeyResponse{
		ID:        k.ID,
		TenantID:  k.TenantID,
		UserID:    k.UserID,
		Name:      k.Name,
		KeyPrefix: k.KeyPrefix,
		Scopes:    k.Scopes,
		CreatedAt: k.CreatedAt,
		LastUsed:  k.LastUsed,
	}
}

func mapTenantError(err error) error {
	switch {
	case errors.Is(err, tenant.ErrOrgNotFound):
		return apierr.NotFound("org")
	case errors.Is(err, tenant.ErrWorkspaceNotFound):
		return apierr.NotFound("workspace")
	case errors.Is(err, tenant.ErrSlugAlreadyTaken):
		return apierr.Conflict(err.Error())
	case errors.Is(err, tenant.ErrInvalidName), errors.Is(err, tenant.ErrInvalidSlug):
		return apierr.BadRequest(err.Error())
	case errors.Is(err, tenant.ErrMemberNotFound):
		return apierr.NotFound("workspace member")
	case errors.Is(err, tenant.ErrMemberAlreadyExists):
		return apierr.Conflict(err.Error())
	case errors.Is(err, tenant.ErrInvalidRole):
		return apierr.BadRequest(err.Error())
	case errors.Is(err, tenant.ErrAPIKeyNotFound):
		return apierr.NotFound("api key")
	case errors.Is(err, tenant.ErrWorkspaceAPIKeyNotFound):
		return apierr.NotFound("workspace api key")
	case errors.Is(err, tenant.ErrInvalidScope):
		return apierr.BadRequest(err.Error())
	default:
		return apierr.Internal(err)
	}
}
