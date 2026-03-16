package http

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/greenlab/ingestion/internal/application"
	"github.com/greenlab/ingestion/internal/domain"
	"github.com/greenlab/shared/pkg/apierr"
	"github.com/greenlab/shared/pkg/response"
	"github.com/greenlab/shared/pkg/validator"
)

// ingestService is the local interface the handler depends on.
type ingestService interface {
	Ingest(ctx context.Context, in application.IngestInput) error
	IngestBatch(ctx context.Context, readings []application.IngestInput) error
}

type Handler struct {
	svc    ingestService
	logger *slog.Logger
}

func NewHandler(svc ingestService, logger *slog.Logger) *Handler {
	return &Handler{svc: svc, logger: logger}
}

// Health godoc
// @Summary      Health check
// @Tags         health
// @Produce      json
// @Success      200  {object}  map[string]interface{}
// @Router       /health [get]
func (h *Handler) Health(c *gin.Context) {
	response.OK(c, gin.H{"status": "ok"})
}

// Ingest godoc
// @Summary      Ingest a single telemetry reading
// @Tags         ingestion
// @Accept       json
// @Produce      json
// @Param        channel_id  path      string         true  "Channel ID"
// @Param        request     body      IngestRequest  true  "Telemetry reading"
// @Success      201         {object}  IngestResponse
// @Failure      400         {object}  map[string]interface{}
// @Failure      401         {object}  map[string]interface{}
// @Failure      403         {object}  map[string]interface{}
// @Failure      503         {object}  map[string]interface{}
// @Security     ApiKeyAuth
// @Router       /v1/channels/{channel_id}/data [post]
func (h *Handler) Ingest(c *gin.Context) {
	var req IngestRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, apierr.BadRequest(err.Error()))
		return
	}
	if err := validator.Validate(&req); err != nil {
		response.ValidationError(c, err)
		return
	}

	pathChannelID := c.Param("channel_id")

	deviceIDStr, ok := contextString(c, "device_id")
	if !ok {
		h.logger.ErrorContext(c.Request.Context(), "device_id missing from auth context")
		response.Error(c, apierr.ErrInternalServerError)
		return
	}
	authChannelID, ok := contextString(c, "channel_id")
	if !ok {
		h.logger.ErrorContext(c.Request.Context(), "channel_id missing from auth context")
		response.Error(c, apierr.ErrInternalServerError)
		return
	}

	if pathChannelID != authChannelID {
		response.Error(c, apierr.New(http.StatusForbidden, "forbidden", "channel_id in path does not match authenticated channel"))
		return
	}

	if err := h.svc.Ingest(c.Request.Context(), application.IngestInput{
		ChannelID: authChannelID,
		DeviceID:  deviceIDStr,
		Fields:    req.Fields,
		Tags:      req.Tags,
		Timestamp: req.Timestamp,
	}); err != nil {
		if isDomainValidationError(err) {
			response.Error(c, apierr.BadRequest(err.Error()))
			return
		}
		h.logger.ErrorContext(c.Request.Context(), "ingest publish failed", "error", err)
		response.Error(c, apierr.New(http.StatusServiceUnavailable, "service_unavailable", "failed to publish reading"))
		return
	}

	response.Created(c, IngestResponse{Accepted: 1, WrittenAt: time.Now().UTC()})
}

// BulkIngest godoc
// @Summary      Ingest a batch of telemetry readings
// @Tags         ingestion
// @Accept       json
// @Produce      json
// @Param        channel_id  path      string            true  "Channel ID"
// @Param        request     body      BulkIngestRequest true  "Batch of readings"
// @Success      201         {object}  IngestResponse
// @Failure      400         {object}  map[string]interface{}
// @Failure      401         {object}  map[string]interface{}
// @Failure      403         {object}  map[string]interface{}
// @Failure      503         {object}  map[string]interface{}
// @Security     ApiKeyAuth
// @Router       /v1/channels/{channel_id}/data/bulk [post]
func (h *Handler) BulkIngest(c *gin.Context) {
	var req BulkIngestRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, apierr.BadRequest(err.Error()))
		return
	}
	if err := validator.Validate(&req); err != nil {
		response.ValidationError(c, err)
		return
	}

	pathChannelID := c.Param("channel_id")

	deviceIDStr, ok := contextString(c, "device_id")
	if !ok {
		h.logger.ErrorContext(c.Request.Context(), "device_id missing from auth context")
		response.Error(c, apierr.ErrInternalServerError)
		return
	}
	authChannelID, ok := contextString(c, "channel_id")
	if !ok {
		h.logger.ErrorContext(c.Request.Context(), "channel_id missing from auth context")
		response.Error(c, apierr.ErrInternalServerError)
		return
	}

	if pathChannelID != authChannelID {
		response.Error(c, apierr.New(http.StatusForbidden, "forbidden", "channel_id in path does not match authenticated channel"))
		return
	}

	inputs := make([]application.IngestInput, len(req.Readings))
	for i, r := range req.Readings {
		inputs[i] = application.IngestInput{
			ChannelID: authChannelID,
			DeviceID:  deviceIDStr,
			Fields:    r.Fields,
			Tags:      r.Tags,
			Timestamp: r.Timestamp,
		}
	}

	if err := h.svc.IngestBatch(c.Request.Context(), inputs); err != nil {
		if isDomainValidationError(err) {
			response.Error(c, apierr.BadRequest(err.Error()))
			return
		}
		h.logger.ErrorContext(c.Request.Context(), "bulk ingest publish failed", "error", err)
		response.Error(c, apierr.New(http.StatusServiceUnavailable, "service_unavailable", "failed to publish readings"))
		return
	}

	response.Created(c, IngestResponse{Accepted: len(req.Readings), WrittenAt: time.Now().UTC()})
}

// isDomainValidationError reports whether err is a domain input validation error
// that should be surfaced to the caller as a 400.
func isDomainValidationError(err error) bool {
	return errors.Is(err, domain.ErrInvalidChannelID) ||
		errors.Is(err, domain.ErrEmptyFields) ||
		errors.Is(err, domain.ErrTimestampTooOld) ||
		errors.Is(err, domain.ErrTimestampFuture)
}

// contextString extracts a non-empty string value from the Gin context.
// Returns ("", false) if the key is absent or the value is not a non-empty string.
func contextString(c *gin.Context, key string) (string, bool) {
	v, exists := c.Get(key)
	if !exists {
		return "", false
	}
	s, ok := v.(string)
	return s, ok && s != ""
}
