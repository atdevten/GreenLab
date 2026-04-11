package http

import (
	"context"
	"errors"

	"github.com/gin-gonic/gin"
	"github.com/greenlab/device-registry/internal/application"
	"github.com/greenlab/device-registry/internal/domain/channel"
	"github.com/greenlab/device-registry/internal/domain/device"
	"github.com/greenlab/device-registry/internal/domain/field"
	"github.com/greenlab/shared/pkg/apierr"
	"github.com/greenlab/shared/pkg/response"
	"github.com/greenlab/shared/pkg/validator"
)

type provisionService interface {
	Provision(ctx context.Context, in application.ProvisionInput) (*application.ProvisionResult, error)
}

// ProvisionHandler handles the atomic device + channel + fields endpoint.
type ProvisionHandler struct {
	svc provisionService
}

// NewProvisionHandler constructs a ProvisionHandler.
func NewProvisionHandler(svc provisionService) *ProvisionHandler {
	return &ProvisionHandler{svc: svc}
}

func mapProvisionError(err error) error {
	switch {
	case errors.Is(err, device.ErrInvalidName), errors.Is(err, device.ErrInvalidStatus):
		return apierr.BadRequest(err.Error())
	case errors.Is(err, channel.ErrInvalidName), errors.Is(err, channel.ErrInvalidVisibility):
		return apierr.BadRequest(err.Error())
	case errors.Is(err, channel.ErrChannelNotFound):
		return apierr.NotFound("channel")
	case errors.Is(err, field.ErrInvalidName), errors.Is(err, field.ErrInvalidPosition), errors.Is(err, field.ErrInvalidFieldType):
		return apierr.BadRequest(err.Error())
	default:
		return apierr.Internal(err)
	}
}

// Provision godoc
// @Summary      Atomically create a device, channel, and fields
// @Tags         devices
// @Accept       json
// @Produce      json
// @Param        request  body      ProvisionRequest   true  "Provision details"
// @Success      201      {object}  ProvisionResponse
// @Failure      400      {object}  map[string]interface{}
// @Security     BearerAuth
// @Router       /api/v1/devices/provision [post]
func (h *ProvisionHandler) Provision(c *gin.Context) {
	var req ProvisionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, apierr.BadRequest(err.Error()))
		return
	}
	if err := validator.Validate(&req); err != nil {
		response.ValidationError(c, err)
		return
	}

	// Exactly one of channel or channel_id must be provided.
	if (req.Channel == nil) == (req.ChannelID == nil) {
		response.Error(c, apierr.BadRequest("provide either 'channel' or 'channel_id', not both"))
		return
	}

	fields := make([]application.ProvisionFieldInput, len(req.Fields))
	for i, f := range req.Fields {
		fields[i] = application.ProvisionFieldInput{
			Name:      f.Name,
			Label:     f.Label,
			Unit:      f.Unit,
			FieldType: f.FieldType,
			Position:  f.Position,
		}
	}

	in := application.ProvisionInput{
		Device: application.ProvisionDeviceInput{
			WorkspaceID: req.Device.WorkspaceID,
			Name:        req.Device.Name,
			Description: req.Device.Description,
		},
		Fields: fields,
	}

	if req.ChannelID != nil {
		in.ExistingChannelID = *req.ChannelID
	} else {
		in.Channel = application.ProvisionChannelInput{
			Name:        req.Channel.Name,
			Description: req.Channel.Description,
			Visibility:  req.Channel.Visibility,
		}
	}

	result, err := h.svc.Provision(c.Request.Context(), in)
	if err != nil {
		response.Error(c, mapProvisionError(err))
		return
	}

	fieldResponses := make([]*FieldResponse, len(result.Fields))
	for i, f := range result.Fields {
		fieldResponses[i] = toFieldResponse(f)
	}

	response.Created(c, &ProvisionResponse{
		Device:  toDeviceResponse(result.Device, true),
		Channel: toChannelResponse(result.Channel),
		Fields:  fieldResponses,
	})
}
