package http

import (
	"context"
	"errors"

	"github.com/gin-gonic/gin"
	"github.com/greenlab/device-registry/internal/application"
	"github.com/greenlab/device-registry/internal/domain/device"
	"github.com/greenlab/shared/pkg/apierr"
	"github.com/greenlab/shared/pkg/response"
)

// internalService is the interface the internal handler depends on.
type internalService interface {
	ValidateAPIKey(ctx context.Context, apiKey, channelID string) (application.ValidateAPIKeyResult, error)
	ResolveChannelByAPIKey(ctx context.Context, apiKey string) (application.ResolveChannelResult, error)
}

// InternalHandler serves machine-to-machine endpoints (no JWT auth).
type InternalHandler struct {
	svc internalService
}

func NewInternalHandler(svc internalService) *InternalHandler {
	return &InternalHandler{svc: svc}
}

// validateAPIKeyRequest is the JSON body for POST /internal/validate-api-key.
type validateAPIKeyRequest struct {
	APIKey    string `json:"api_key"    binding:"required"`
	ChannelID string `json:"channel_id" binding:"required"`
}

// fieldEntryResponse is the per-field schema entry in the response.
type fieldEntryResponse struct {
	Index uint8  `json:"index"`
	Name  string `json:"name"`
	Type  string `json:"type"`
}

// validateAPIKeyResponse is the response body for a successful validation.
type validateAPIKeyResponse struct {
	DeviceID      string               `json:"device_id"`
	Fields        []fieldEntryResponse `json:"fields"`
	SchemaVersion uint32               `json:"schema_version"`
}

// schemaResponse is the response body for GET /v1/channels/:id/schema.
type schemaResponse struct {
	Fields        []fieldEntryResponse `json:"fields"`
	SchemaVersion uint32               `json:"schema_version"`
}

// resolveChannelResponse is the response body for GET /internal/resolve-channel.
type resolveChannelResponse struct {
	DeviceID      string               `json:"device_id"`
	ChannelID     string               `json:"channel_id"`
	Fields        []fieldEntryResponse `json:"fields"`
	SchemaVersion uint32               `json:"schema_version"`
}

// GetChannelSchema godoc
// @Summary      Get the compact-format field schema for a channel
// @Tags         schema
// @Produce      json
// @Param        id  path  string  true  "Channel ID"
// @Success      200  {object}  schemaResponse
// @Failure      401  {object}  map[string]interface{}
// @Failure      404  {object}  map[string]interface{}
// @Security     ApiKeyAuth
// @Router       /v1/channels/{id}/schema [get]
func (h *InternalHandler) GetChannelSchema(c *gin.Context) {
	apiKey := c.GetHeader("X-API-Key")
	if apiKey == "" {
		apiKey = c.Query("api_key")
	}
	if apiKey == "" {
		response.Error(c, apierr.Unauthorized("missing API key"))
		return
	}

	channelID := c.Param("id")
	result, err := h.svc.ValidateAPIKey(c.Request.Context(), apiKey, channelID)
	if err != nil {
		if errors.Is(err, device.ErrDeviceNotFound) {
			response.Error(c, apierr.Unauthorized("invalid API key or channel"))
			return
		}
		response.Error(c, apierr.ErrInternalServerError)
		return
	}

	fields := make([]fieldEntryResponse, len(result.Fields))
	for i, f := range result.Fields {
		fields[i] = fieldEntryResponse{Index: f.Index, Name: f.Name, Type: f.Type}
	}

	response.OK(c, schemaResponse{
		Fields:        fields,
		SchemaVersion: result.SchemaVersion,
	})
}

// ResolveChannel godoc
// @Summary      Resolve the first channel owned by the device for a given API key (internal)
// @Tags         internal
// @Produce      json
// @Param        api_key  query     string  true  "Device API key"
// @Success      200      {object}  resolveChannelResponse
// @Failure      400      {object}  map[string]interface{}
// @Failure      401      {object}  map[string]interface{}
// @Router       /internal/resolve-channel [get]
func (h *InternalHandler) ResolveChannel(c *gin.Context) {
	apiKey := c.Query("api_key")
	if apiKey == "" {
		response.Error(c, apierr.BadRequest("missing api_key query parameter"))
		return
	}

	result, err := h.svc.ResolveChannelByAPIKey(c.Request.Context(), apiKey)
	if err != nil {
		if errors.Is(err, device.ErrDeviceNotFound) {
			response.Error(c, apierr.Unauthorized("invalid API key"))
			return
		}
		response.Error(c, apierr.ErrInternalServerError)
		return
	}

	fields := make([]fieldEntryResponse, len(result.Fields))
	for i, f := range result.Fields {
		fields[i] = fieldEntryResponse{Index: f.Index, Name: f.Name, Type: f.Type}
	}

	response.OK(c, resolveChannelResponse{
		DeviceID:      result.DeviceID,
		ChannelID:     result.ChannelID,
		Fields:        fields,
		SchemaVersion: result.SchemaVersion,
	})
}

// ValidateAPIKey godoc
// @Summary      Validate an API key + channel combination (internal)
// @Tags         internal
// @Accept       json
// @Produce      json
// @Param        request  body      validateAPIKeyRequest    true  "API key and channel ID"
// @Success      200      {object}  validateAPIKeyResponse
// @Failure      400      {object}  map[string]interface{}
// @Failure      401      {object}  map[string]interface{}
// @Router       /internal/validate-api-key [post]
func (h *InternalHandler) ValidateAPIKey(c *gin.Context) {
	var req validateAPIKeyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, apierr.BadRequest(err.Error()))
		return
	}

	result, err := h.svc.ValidateAPIKey(c.Request.Context(), req.APIKey, req.ChannelID)
	if err != nil {
		if errors.Is(err, device.ErrDeviceNotFound) {
			response.Error(c, apierr.Unauthorized("invalid API key or channel"))
			return
		}
		response.Error(c, apierr.ErrInternalServerError)
		return
	}

	fields := make([]fieldEntryResponse, len(result.Fields))
	for i, f := range result.Fields {
		fields[i] = fieldEntryResponse{Index: f.Index, Name: f.Name, Type: f.Type}
	}

	response.OK(c, validateAPIKeyResponse{
		DeviceID:      result.DeviceID,
		Fields:        fields,
		SchemaVersion: result.SchemaVersion,
	})
}
