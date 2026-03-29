package http

import (
	"context"
	"errors"
	"io"
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

const (
	ctJSON      = "application/json"
	ctMsgPack   = "application/msgpack"
	ctProtobuf  = "application/x-protobuf"
	ctBinary    = "application/x-thingspeak-binary"
	ctOJSON     = "application/x-greenlab-ojson"
	maxBodyJSON = 1 << 20  // 1 MB
	maxBodyMsgP = 1 << 20  // 1 MB
	maxBodyBin  = 32       // 32 bytes
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
	schema, ok := contextDeviceSchema(c)
	if !ok {
		h.logger.ErrorContext(c.Request.Context(), "device_schema missing from auth context")
		response.Error(c, apierr.ErrInternalServerError)
		return
	}

	channelID := c.Param("channel_id")
	ct := c.ContentType()

	switch ct {
	case ctJSON, "":
		h.ingestJSON(c, channelID, schema.DeviceID)
	case ctOJSON:
		h.ingestCompact(c, channelID, schema, newOJSONDeserializer(), maxBodyJSON)
	case ctMsgPack:
		h.ingestCompact(c, channelID, schema, newMsgPackDeserializer(), maxBodyMsgP)
	case ctBinary:
		h.ingestCompact(c, channelID, schema, newBinaryDeserializer(schema.DeviceID), maxBodyBin)
	case ctProtobuf:
		// TODO: protobuf support — requires proto code generation tooling (TODO-027)
		response.Error(c, apierr.New(http.StatusNotImplemented, "not_implemented", "protobuf support coming soon"))
	default:
		response.Error(c, apierr.New(http.StatusUnsupportedMediaType, "unsupported_media_type", "unsupported Content-Type"))
	}
}

func (h *Handler) ingestJSON(c *gin.Context, channelID, deviceID string) {
	var req IngestRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, apierr.BadRequest(err.Error()))
		return
	}
	if err := validator.Validate(&req); err != nil {
		response.ValidationError(c, err)
		return
	}

	if err := h.svc.Ingest(c.Request.Context(), application.IngestInput{
		ChannelID:       channelID,
		DeviceID:        deviceID,
		Fields:          req.Fields,
		FieldTimestamps: req.FieldTimestamps,
		Tags:            req.Tags,
		Timestamp:       req.Timestamp,
	}); err != nil {
		errorToHTTPResponse(c, err, h.logger)
		return
	}

	response.Created(c, IngestResponse{Accepted: 1, WrittenAt: time.Now().UTC()})
}

func (h *Handler) ingestCompact(c *gin.Context, channelID string, schema domain.DeviceSchema, d Deserializer, maxBody int64) {
	body, err := io.ReadAll(io.LimitReader(c.Request.Body, maxBody+1))
	if err != nil {
		response.Error(c, apierr.BadRequest(domain.ErrBodyReadError.Error()))
		return
	}
	if int64(len(body)) > maxBody {
		response.Error(c, apierr.New(http.StatusRequestEntityTooLarge, "payload_too_large", domain.ErrPayloadTooLarge.Error()))
		return
	}

	inputs, err := deserializeCompact(body, schema, d)
	if err != nil {
		errorToHTTPResponse(c, err, h.logger)
		return
	}

	for i := range inputs {
		inputs[i].ChannelID = channelID
		inputs[i].DeviceID = schema.DeviceID
	}

	if err := h.svc.IngestBatch(c.Request.Context(), inputs); err != nil {
		errorToHTTPResponse(c, err, h.logger)
		return
	}

	response.Created(c, IngestResponse{Accepted: len(inputs), WrittenAt: time.Now().UTC()})
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

	channelID := c.Param("channel_id")

	schema, ok := contextDeviceSchema(c)
	if !ok {
		h.logger.ErrorContext(c.Request.Context(), "device_schema missing from auth context")
		response.Error(c, apierr.ErrInternalServerError)
		return
	}

	inputs := make([]application.IngestInput, len(req.Readings))
	for i, r := range req.Readings {
		inputs[i] = application.IngestInput{
			ChannelID:       channelID,
			DeviceID:        schema.DeviceID,
			Fields:          r.Fields,
			FieldTimestamps: r.FieldTimestamps,
			Tags:            r.Tags,
			Timestamp:       r.Timestamp,
		}
	}

	if err := h.svc.IngestBatch(c.Request.Context(), inputs); err != nil {
		errorToHTTPResponse(c, err, h.logger)
		return
	}

	response.Created(c, IngestResponse{Accepted: len(req.Readings), WrittenAt: time.Now().UTC()})
}

// errorToHTTPResponse maps domain errors to appropriate HTTP responses.
func errorToHTTPResponse(c *gin.Context, err error, logger *slog.Logger) {
	switch {
	case errors.Is(err, domain.ErrSchemaMismatch):
		response.Error(c, apierr.New(http.StatusConflict, "schema_version_mismatch", err.Error()))
	case errors.Is(err, domain.ErrDeviceIDMismatch):
		response.Error(c, apierr.New(http.StatusForbidden, "device_id_mismatch", err.Error()))
	case isDomainValidationError(err):
		response.Error(c, apierr.BadRequest(err.Error()))
	case errors.Is(err, domain.ErrPayloadTooLarge):
		response.Error(c, apierr.New(http.StatusRequestEntityTooLarge, "payload_too_large", err.Error()))
	case errors.Is(err, domain.ErrUnknownFieldIndex),
		errors.Is(err, domain.ErrMissingSchemaVersion),
		errors.Is(err, domain.ErrCRCMismatch),
		errors.Is(err, domain.ErrInvalidFrameLength),
		errors.Is(err, domain.ErrTSDeltaInvalid),
		errors.Is(err, domain.ErrBodyReadError):
		response.Error(c, apierr.BadRequest(err.Error()))
	default:
		logger.ErrorContext(c.Request.Context(), "ingest failed", "error", err)
		response.Error(c, apierr.New(http.StatusServiceUnavailable, "service_unavailable", "failed to publish reading"))
	}
}

// isDomainValidationError reports whether err is a domain input validation error
// that should be surfaced to the caller as a 400.
func isDomainValidationError(err error) bool {
	return errors.Is(err, domain.ErrInvalidChannelID) ||
		errors.Is(err, domain.ErrEmptyFields) ||
		errors.Is(err, domain.ErrTimestampTooOld) ||
		errors.Is(err, domain.ErrTimestampFuture)
}

// contextDeviceSchema extracts a domain.DeviceSchema from the Gin context.
// Returns (zero, false) if the key is absent or of the wrong type.
func contextDeviceSchema(c *gin.Context) (domain.DeviceSchema, bool) {
	v, exists := c.Get("device_schema")
	if !exists {
		return domain.DeviceSchema{}, false
	}
	schema, ok := v.(domain.DeviceSchema)
	return schema, ok
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
