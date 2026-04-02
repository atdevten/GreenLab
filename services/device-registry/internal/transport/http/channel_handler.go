package http

import (
	"context"
	"errors"

	"github.com/gin-gonic/gin"
	"github.com/greenlab/device-registry/internal/application"
	"github.com/greenlab/device-registry/internal/domain/channel"
	"github.com/greenlab/shared/pkg/apierr"
	"github.com/greenlab/shared/pkg/pagination"
	"github.com/greenlab/shared/pkg/response"
	"github.com/greenlab/shared/pkg/validator"
)

type channelService interface {
	CreateChannel(ctx context.Context, in application.CreateChannelInput) (*channel.Channel, error)
	GetChannel(ctx context.Context, id string) (*channel.Channel, error)
	ListChannels(ctx context.Context, workspaceID string, limit, offset int) ([]*channel.Channel, int64, error)
	ListChannelsByDevice(ctx context.Context, deviceID string, limit, offset int) ([]*channel.Channel, int64, error)
	UpdateChannel(ctx context.Context, id string, in application.UpdateChannelInput) (*channel.Channel, error)
	DeleteChannel(ctx context.Context, id string) error
}

type ChannelHandler struct {
	svc channelService
}

func NewChannelHandler(svc channelService) *ChannelHandler {
	return &ChannelHandler{svc: svc}
}

func mapChannelError(err error) error {
	switch {
	case errors.Is(err, channel.ErrChannelNotFound):
		return apierr.NotFound("channel")
	case errors.Is(err, channel.ErrInvalidName),
		errors.Is(err, channel.ErrInvalidVisibility),
		errors.Is(err, channel.ErrInvalidRetention):
		return apierr.BadRequest(err.Error())
	default:
		return apierr.Internal(err)
	}
}

// CreateChannel godoc
// @Summary      Create a new channel
// @Tags         channels
// @Accept       json
// @Produce      json
// @Param        request  body      CreateChannelRequest  true  "Channel details"
// @Success      201      {object}  ChannelResponse
// @Failure      400      {object}  map[string]interface{}
// @Security     BearerAuth
// @Router       /api/v1/channels [post]
func (h *ChannelHandler) CreateChannel(c *gin.Context) {
	var req CreateChannelRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, apierr.BadRequest(err.Error()))
		return
	}
	if err := validator.Validate(&req); err != nil {
		response.ValidationError(c, err)
		return
	}
	ch, err := h.svc.CreateChannel(c.Request.Context(), application.CreateChannelInput{
		WorkspaceID:   req.WorkspaceID,
		DeviceID:      req.DeviceID,
		Name:          req.Name,
		Description:   req.Description,
		Visibility:    req.Visibility,
		RetentionDays: req.RetentionDays,
	})
	if err != nil {
		response.Error(c, mapChannelError(err))
		return
	}
	response.Created(c, toChannelResponse(ch))
}

// GetChannel godoc
// @Summary      Get a channel by ID
// @Tags         channels
// @Produce      json
// @Param        id  path      string  true  "Channel ID"
// @Success      200  {object}  ChannelResponse
// @Failure      404  {object}  map[string]interface{}
// @Security     BearerAuth
// @Router       /api/v1/channels/{id} [get]
func (h *ChannelHandler) GetChannel(c *gin.Context) {
	ch, err := h.svc.GetChannel(c.Request.Context(), c.Param("id"))
	if err != nil {
		response.Error(c, mapChannelError(err))
		return
	}
	response.OK(c, toChannelResponse(ch))
}

// ListChannels godoc
// @Summary      List channels in a workspace or for a device
// @Tags         channels
// @Produce      json
// @Param        workspace_id  query     string  false  "Workspace ID (required if device_id not provided)"
// @Param        device_id     query     string  false  "Device ID filter"
// @Param        limit         query     int     false  "Page size"
// @Param        offset        query     int     false  "Page offset"
// @Success      200           {array}   ChannelResponse
// @Failure      400           {object}  map[string]interface{}
// @Security     BearerAuth
// @Router       /api/v1/channels [get]
func (h *ChannelHandler) ListChannels(c *gin.Context) {
	page := pagination.ParseOffset(c)

	deviceID := c.Query("device_id")
	if deviceID != "" {
		channels, total, err := h.svc.ListChannelsByDevice(c.Request.Context(), deviceID, page.Limit, page.Offset())
		if err != nil {
			response.Error(c, mapChannelError(err))
			return
		}
		items := make([]*ChannelResponse, len(channels))
		for i, ch := range channels {
			items[i] = toChannelResponse(ch)
		}
		response.OKWithMeta(c, items, pagination.NewOffsetResult(items, total, page))
		return
	}

	wsID := c.Query("workspace_id")
	if wsID == "" {
		response.Error(c, apierr.BadRequest("workspace_id is required"))
		return
	}
	channels, total, err := h.svc.ListChannels(c.Request.Context(), wsID, page.Limit, page.Offset())
	if err != nil {
		response.Error(c, mapChannelError(err))
		return
	}
	items := make([]*ChannelResponse, len(channels))
	for i, ch := range channels {
		items[i] = toChannelResponse(ch)
	}
	response.OKWithMeta(c, items, pagination.NewOffsetResult(items, total, page))
}

// UpdateChannel godoc
// @Summary      Update a channel
// @Tags         channels
// @Accept       json
// @Produce      json
// @Param        id       path      string               true  "Channel ID"
// @Param        request  body      UpdateChannelRequest  true  "Update fields"
// @Success      200      {object}  ChannelResponse
// @Failure      400      {object}  map[string]interface{}
// @Failure      404      {object}  map[string]interface{}
// @Security     BearerAuth
// @Router       /api/v1/channels/{id} [put]
func (h *ChannelHandler) UpdateChannel(c *gin.Context) {
	var req UpdateChannelRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, apierr.BadRequest(err.Error()))
		return
	}
	ch, err := h.svc.UpdateChannel(c.Request.Context(), c.Param("id"), application.UpdateChannelInput{
		Name: req.Name, Description: req.Description, Visibility: req.Visibility,
		RetentionDays: req.RetentionDays,
	})
	if err != nil {
		response.Error(c, mapChannelError(err))
		return
	}
	response.OK(c, toChannelResponse(ch))
}

// DeleteChannel godoc
// @Summary      Delete a channel
// @Tags         channels
// @Param        id  path  string  true  "Channel ID"
// @Success      204  "No Content"
// @Failure      404  {object}  map[string]interface{}
// @Security     BearerAuth
// @Router       /api/v1/channels/{id} [delete]
func (h *ChannelHandler) DeleteChannel(c *gin.Context) {
	if err := h.svc.DeleteChannel(c.Request.Context(), c.Param("id")); err != nil {
		response.Error(c, mapChannelError(err))
		return
	}
	response.NoContent(c)
}

func toChannelResponse(ch *channel.Channel) *ChannelResponse {
	r := &ChannelResponse{
		ID: ch.ID.String(), WorkspaceID: ch.WorkspaceID.String(),
		Name: ch.Name, Description: ch.Description,
		Visibility:    string(ch.Visibility),
		RetentionDays: ch.RetentionDays,
		CreatedAt:     ch.CreatedAt, UpdatedAt: ch.UpdatedAt,
	}
	if ch.DeviceID != nil {
		s := ch.DeviceID.String()
		r.DeviceID = &s
	}
	return r
}
