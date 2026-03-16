package http

import (
	"context"
	"errors"

	"github.com/gin-gonic/gin"
	"github.com/greenlab/device-registry/internal/application"
	"github.com/greenlab/device-registry/internal/domain/device"
	"github.com/greenlab/shared/pkg/apierr"
	"github.com/greenlab/shared/pkg/pagination"
	"github.com/greenlab/shared/pkg/response"
	"github.com/greenlab/shared/pkg/validator"
)

type deviceService interface {
	CreateDevice(ctx context.Context, in application.CreateDeviceInput) (*device.Device, error)
	GetDevice(ctx context.Context, id string) (*device.Device, error)
	ListDevices(ctx context.Context, workspaceID string, limit, offset int) ([]*device.Device, int64, error)
	UpdateDevice(ctx context.Context, id string, in application.UpdateDeviceInput) (*device.Device, error)
	RotateAPIKey(ctx context.Context, id string) (*device.Device, error)
	DeleteDevice(ctx context.Context, id string) error
}

type DeviceHandler struct {
	svc deviceService
}

func NewDeviceHandler(svc deviceService) *DeviceHandler {
	return &DeviceHandler{svc: svc}
}

func mapDeviceError(err error) error {
	switch {
	case errors.Is(err, device.ErrDeviceNotFound):
		return apierr.NotFound("device")
	case errors.Is(err, device.ErrInvalidName), errors.Is(err, device.ErrInvalidStatus):
		return apierr.BadRequest(err.Error())
	default:
		return apierr.Internal(err)
	}
}

// Health godoc
// @Summary      Health check
// @Tags         health
// @Produce      json
// @Success      200  {object}  map[string]interface{}
// @Router       /health [get]
func (h *DeviceHandler) Health(c *gin.Context) {
	response.OK(c, gin.H{"status": "ok"})
}

// CreateDevice godoc
// @Summary      Register a new device
// @Tags         devices
// @Accept       json
// @Produce      json
// @Param        request  body      CreateDeviceRequest  true  "Device details"
// @Success      201      {object}  DeviceResponse
// @Failure      400      {object}  map[string]interface{}
// @Security     BearerAuth
// @Router       /api/v1/devices [post]
func (h *DeviceHandler) CreateDevice(c *gin.Context) {
	var req CreateDeviceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, apierr.BadRequest(err.Error()))
		return
	}
	if err := validator.Validate(&req); err != nil {
		response.ValidationError(c, err)
		return
	}
	d, err := h.svc.CreateDevice(c.Request.Context(), application.CreateDeviceInput{
		WorkspaceID: req.WorkspaceID, Name: req.Name, Description: req.Description,
	})
	if err != nil {
		response.Error(c, mapDeviceError(err))
		return
	}
	response.Created(c, toDeviceResponse(d, true))
}

// GetDevice godoc
// @Summary      Get a device by ID
// @Tags         devices
// @Produce      json
// @Param        id  path      string  true  "Device ID"
// @Success      200  {object}  DeviceResponse
// @Failure      404  {object}  map[string]interface{}
// @Security     BearerAuth
// @Router       /api/v1/devices/{id} [get]
func (h *DeviceHandler) GetDevice(c *gin.Context) {
	d, err := h.svc.GetDevice(c.Request.Context(), c.Param("id"))
	if err != nil {
		response.Error(c, mapDeviceError(err))
		return
	}
	response.OK(c, toDeviceResponse(d, false))
}

// ListDevices godoc
// @Summary      List devices in a workspace
// @Tags         devices
// @Produce      json
// @Param        workspace_id  query     string  true   "Workspace ID"
// @Param        limit         query     int     false  "Page size"
// @Param        offset        query     int     false  "Page offset"
// @Success      200           {array}   DeviceResponse
// @Failure      400           {object}  map[string]interface{}
// @Security     BearerAuth
// @Router       /api/v1/devices [get]
func (h *DeviceHandler) ListDevices(c *gin.Context) {
	wsID := c.Query("workspace_id")
	if wsID == "" {
		response.Error(c, apierr.BadRequest("workspace_id is required"))
		return
	}
	page := pagination.ParseOffset(c)
	devices, total, err := h.svc.ListDevices(c.Request.Context(), wsID, page.Limit, page.Offset())
	if err != nil {
		response.Error(c, mapDeviceError(err))
		return
	}
	items := make([]*DeviceResponse, len(devices))
	for i, d := range devices {
		items[i] = toDeviceResponse(d, false)
	}
	response.OKWithMeta(c, items, pagination.NewOffsetResult(items, total, page))
}

// UpdateDevice godoc
// @Summary      Update a device
// @Tags         devices
// @Accept       json
// @Produce      json
// @Param        id       path      string              true  "Device ID"
// @Param        request  body      UpdateDeviceRequest  true  "Update fields"
// @Success      200      {object}  DeviceResponse
// @Failure      400      {object}  map[string]interface{}
// @Failure      404      {object}  map[string]interface{}
// @Security     BearerAuth
// @Router       /api/v1/devices/{id} [put]
func (h *DeviceHandler) UpdateDevice(c *gin.Context) {
	var req UpdateDeviceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, apierr.BadRequest(err.Error()))
		return
	}
	d, err := h.svc.UpdateDevice(c.Request.Context(), c.Param("id"), application.UpdateDeviceInput{
		Name: req.Name, Description: req.Description, Status: req.Status,
	})
	if err != nil {
		response.Error(c, mapDeviceError(err))
		return
	}
	response.OK(c, toDeviceResponse(d, false))
}

// RotateAPIKey godoc
// @Summary      Rotate the API key for a device
// @Tags         devices
// @Produce      json
// @Param        id  path      string  true  "Device ID"
// @Success      200  {object}  DeviceResponse
// @Failure      404  {object}  map[string]interface{}
// @Security     BearerAuth
// @Router       /api/v1/devices/{id}/rotate-key [post]
func (h *DeviceHandler) RotateAPIKey(c *gin.Context) {
	d, err := h.svc.RotateAPIKey(c.Request.Context(), c.Param("id"))
	if err != nil {
		response.Error(c, mapDeviceError(err))
		return
	}
	response.OK(c, toDeviceResponse(d, true))
}

// DeleteDevice godoc
// @Summary      Delete a device
// @Tags         devices
// @Param        id  path  string  true  "Device ID"
// @Success      204  "No Content"
// @Failure      404  {object}  map[string]interface{}
// @Security     BearerAuth
// @Router       /api/v1/devices/{id} [delete]
func (h *DeviceHandler) DeleteDevice(c *gin.Context) {
	if err := h.svc.DeleteDevice(c.Request.Context(), c.Param("id")); err != nil {
		response.Error(c, mapDeviceError(err))
		return
	}
	response.NoContent(c)
}

// ListByWorkspace godoc
// @Summary      List devices scoped by workspace URL param
// @Tags         workspaces
// @Produce      json
// @Param        id      path      string  true   "Workspace ID"
// @Param        limit   query     int     false  "Page size"
// @Param        offset  query     int     false  "Page offset"
// @Success      200     {array}   DeviceResponse
// @Failure      400     {object}  map[string]interface{}
// @Security     BearerAuth
// @Router       /api/v1/workspaces/{id}/devices [get]
func (h *DeviceHandler) ListByWorkspace(c *gin.Context) {
	wsID := c.Param("id")
	if wsID == "" {
		response.Error(c, apierr.BadRequest("workspace id is required"))
		return
	}
	page := pagination.ParseOffset(c)
	devices, total, err := h.svc.ListDevices(c.Request.Context(), wsID, page.Limit, page.Offset())
	if err != nil {
		response.Error(c, mapDeviceError(err))
		return
	}
	items := make([]*DeviceResponse, len(devices))
	for i, d := range devices {
		items[i] = toDeviceResponse(d, false)
	}
	response.OKWithMeta(c, items, pagination.NewOffsetResult(items, total, page))
}

func toDeviceResponse(d *device.Device, showKey bool) *DeviceResponse {
	r := &DeviceResponse{
		ID: d.ID.String(), WorkspaceID: d.WorkspaceID.String(),
		Name: d.Name, Description: d.Description,
		Status: string(d.Status), Metadata: d.Metadata,
		LastSeenAt: d.LastSeenAt, CreatedAt: d.CreatedAt, UpdatedAt: d.UpdatedAt,
	}
	if showKey {
		r.APIKey = d.APIKey
	}
	return r
}
