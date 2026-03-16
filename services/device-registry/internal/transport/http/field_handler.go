package http

import (
	"context"
	"errors"

	"github.com/gin-gonic/gin"
	"github.com/greenlab/device-registry/internal/application"
	"github.com/greenlab/device-registry/internal/domain/field"
	"github.com/greenlab/shared/pkg/apierr"
	"github.com/greenlab/shared/pkg/response"
	"github.com/greenlab/shared/pkg/validator"
)

type fieldService interface {
	CreateField(ctx context.Context, in application.CreateFieldInput) (*field.Field, error)
	GetField(ctx context.Context, id string) (*field.Field, error)
	ListFields(ctx context.Context, channelID string) ([]*field.Field, error)
	UpdateField(ctx context.Context, id string, in application.UpdateFieldInput) (*field.Field, error)
	DeleteField(ctx context.Context, id string) error
}

type FieldHandler struct {
	svc fieldService
}

func NewFieldHandler(svc fieldService) *FieldHandler {
	return &FieldHandler{svc: svc}
}

func mapFieldError(err error) error {
	switch {
	case errors.Is(err, field.ErrFieldNotFound):
		return apierr.NotFound("field")
	case errors.Is(err, field.ErrInvalidName), errors.Is(err, field.ErrInvalidPosition), errors.Is(err, field.ErrInvalidFieldType):
		return apierr.BadRequest(err.Error())
	default:
		return apierr.Internal(err)
	}
}

// CreateField godoc
// @Summary      Create a new field on a channel
// @Tags         fields
// @Accept       json
// @Produce      json
// @Param        request  body      CreateFieldRequest  true  "Field details"
// @Success      201      {object}  FieldResponse
// @Failure      400      {object}  map[string]interface{}
// @Security     BearerAuth
// @Router       /api/v1/fields [post]
func (h *FieldHandler) CreateField(c *gin.Context) {
	var req CreateFieldRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, apierr.BadRequest(err.Error()))
		return
	}
	if err := validator.Validate(&req); err != nil {
		response.ValidationError(c, err)
		return
	}
	f, err := h.svc.CreateField(c.Request.Context(), application.CreateFieldInput{
		ChannelID: req.ChannelID, Name: req.Name, Label: req.Label,
		Unit: req.Unit, FieldType: req.FieldType, Position: *req.Position, Description: req.Description,
	})
	if err != nil {
		response.Error(c, mapFieldError(err))
		return
	}
	response.Created(c, toFieldResponse(f))
}

// GetField godoc
// @Summary      Get a field by ID
// @Tags         fields
// @Produce      json
// @Param        id  path      string  true  "Field ID"
// @Success      200  {object}  FieldResponse
// @Failure      404  {object}  map[string]interface{}
// @Security     BearerAuth
// @Router       /api/v1/fields/{id} [get]
func (h *FieldHandler) GetField(c *gin.Context) {
	f, err := h.svc.GetField(c.Request.Context(), c.Param("id"))
	if err != nil {
		response.Error(c, mapFieldError(err))
		return
	}
	response.OK(c, toFieldResponse(f))
}

// ListFields godoc
// @Summary      List fields for a channel
// @Tags         fields
// @Produce      json
// @Param        channel_id  query     string  true  "Channel ID"
// @Success      200         {array}   FieldResponse
// @Failure      400         {object}  map[string]interface{}
// @Security     BearerAuth
// @Router       /api/v1/fields [get]
func (h *FieldHandler) ListFields(c *gin.Context) {
	chID := c.Query("channel_id")
	if chID == "" {
		response.Error(c, apierr.BadRequest("channel_id is required"))
		return
	}
	fields, err := h.svc.ListFields(c.Request.Context(), chID)
	if err != nil {
		response.Error(c, mapFieldError(err))
		return
	}
	items := make([]*FieldResponse, len(fields))
	for i, f := range fields {
		items[i] = toFieldResponse(f)
	}
	response.OK(c, items)
}

// UpdateField godoc
// @Summary      Update a field
// @Tags         fields
// @Accept       json
// @Produce      json
// @Param        id       path      string             true  "Field ID"
// @Param        request  body      UpdateFieldRequest  true  "Update fields"
// @Success      200      {object}  FieldResponse
// @Failure      400      {object}  map[string]interface{}
// @Failure      404      {object}  map[string]interface{}
// @Security     BearerAuth
// @Router       /api/v1/fields/{id} [put]
func (h *FieldHandler) UpdateField(c *gin.Context) {
	var req UpdateFieldRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, apierr.BadRequest(err.Error()))
		return
	}
	f, err := h.svc.UpdateField(c.Request.Context(), c.Param("id"), application.UpdateFieldInput{
		Name: req.Name, Label: req.Label, Unit: req.Unit, Description: req.Description,
	})
	if err != nil {
		response.Error(c, mapFieldError(err))
		return
	}
	response.OK(c, toFieldResponse(f))
}

// DeleteField godoc
// @Summary      Delete a field
// @Tags         fields
// @Param        id  path  string  true  "Field ID"
// @Success      204  "No Content"
// @Failure      404  {object}  map[string]interface{}
// @Security     BearerAuth
// @Router       /api/v1/fields/{id} [delete]
func (h *FieldHandler) DeleteField(c *gin.Context) {
	if err := h.svc.DeleteField(c.Request.Context(), c.Param("id")); err != nil {
		response.Error(c, mapFieldError(err))
		return
	}
	response.NoContent(c)
}

func toFieldResponse(f *field.Field) *FieldResponse {
	return &FieldResponse{
		ID: f.ID.String(), ChannelID: f.ChannelID.String(),
		Name: f.Name, Label: f.Label, Unit: f.Unit,
		FieldType: string(f.FieldType), Position: f.Position,
		Description: f.Description, CreatedAt: f.CreatedAt, UpdatedAt: f.UpdatedAt,
	}
}
