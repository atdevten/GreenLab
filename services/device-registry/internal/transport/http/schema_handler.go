package http

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/greenlab/device-registry/internal/domain/channel"
	"github.com/greenlab/shared/pkg/apierr"
	"github.com/greenlab/shared/pkg/response"
)

const schemaForceDeprecatedTTL = 48 * time.Hour

// schemaDeprecationStore persists force-deprecation markers.
// Implemented by the Redis adapter in infrastructure/redis.
type schemaDeprecationStore interface {
	SetForceDeprecated(ctx context.Context, channelID string) error
}

// schemaChannelGetter fetches a channel so the handler can verify it exists.
type schemaChannelGetter interface {
	GetChannel(ctx context.Context, id string) (*channel.Channel, error)
}

// SchemaHandler serves schema OTA management endpoints.
type SchemaHandler struct {
	channelSvc  schemaChannelGetter
	deprecStore schemaDeprecationStore
}

// NewSchemaHandler creates a SchemaHandler.
func NewSchemaHandler(channelSvc schemaChannelGetter, deprecStore schemaDeprecationStore) *SchemaHandler {
	return &SchemaHandler{channelSvc: channelSvc, deprecStore: deprecStore}
}

// forceDeprecateResponse is returned on a successful force-deprecation request.
type forceDeprecateResponse struct {
	ChannelID  string    `json:"channel_id"`
	Deprecated bool      `json:"deprecated"`
	ExpiresAt  time.Time `json:"expires_at"`
	Note       string    `json:"note"`
}

// ForceDeprecateSchema godoc
// @Summary      Force-deprecate a schema version for a channel
// @Description  Sets a 48-hour deprecation marker in Redis. After this window, devices
//               using the old schema version should receive 410 Gone responses (enforcement
//               in ingestion is future work — see TODO-032).
// @Tags         channels
// @Produce      json
// @Param        id  path      string  true  "Channel ID"
// @Success      200  {object}  forceDeprecateResponse
// @Failure      404  {object}  map[string]interface{}
// @Failure      500  {object}  map[string]interface{}
// @Security     BearerAuth
// @Router       /api/v1/channels/{id}/schema/force-deprecate [post]
func (h *SchemaHandler) ForceDeprecateSchema(c *gin.Context) {
	channelID := c.Param("id")

	// Verify the channel exists before setting the deprecation key.
	if _, err := h.channelSvc.GetChannel(c.Request.Context(), channelID); err != nil {
		response.Error(c, mapChannelError(err))
		return
	}

	if err := h.deprecStore.SetForceDeprecated(c.Request.Context(), channelID); err != nil {
		response.Error(c, apierr.New(http.StatusInternalServerError, "internal_error",
			fmt.Sprintf("failed to set force-deprecation marker: %s", err.Error())))
		return
	}

	expiresAt := time.Now().UTC().Add(schemaForceDeprecatedTTL)
	response.OK(c, forceDeprecateResponse{
		ChannelID:  channelID,
		Deprecated: true,
		ExpiresAt:  expiresAt,
		Note: "Devices using the deprecated schema_version will receive 410 Gone on compact-format " +
			"ingestion requests until the deprecation marker expires.",
	})
}
