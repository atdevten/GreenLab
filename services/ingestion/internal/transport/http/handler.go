package http

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/greenlab/ingestion/internal/application"
	"github.com/greenlab/ingestion/internal/domain"
	"github.com/greenlab/shared/pkg/apierr"
	"github.com/greenlab/shared/pkg/middleware"
	"github.com/greenlab/shared/pkg/response"
	"github.com/greenlab/shared/pkg/validator"
)

const (
	ctJSON      = "application/json"
	ctMsgPack   = "application/msgpack"
	ctProtobuf  = "application/x-protobuf"
	ctBinary    = "application/x-thingspeak-binary"
	ctOJSON     = "application/x-greenlab-ojson"
	maxBodyJSON = 1 << 20 // 1 MB
	maxBodyMsgP = 1 << 20 // 1 MB
	maxBodyBin  = 32      // 32 bytes
	maxBulkSize = 1000 // max readings per bulk request
)

// ingestService is the local interface the handler depends on.
type ingestService interface {
	Ingest(ctx context.Context, in application.IngestInput) error
	IngestBatch(ctx context.Context, readings []application.IngestInput) error
	IngestReplay(ctx context.Context, inputs []application.IngestInput, windowDays int, dlq application.ReplayDLQWriter) error
}

// schemaACKStore records per-device schema version acknowledgements.
// A nil value disables ACK recording (fail-open).
type schemaACKStore interface {
	RecordACK(ctx context.Context, channelID, deviceID string, version uint32) error
}

type Handler struct {
	svc        ingestService
	logger     *slog.Logger
	ackStore   schemaACKStore          // nil = disabled
	replayDLQ  application.ReplayDLQWriter // nil = DLQ disabled (fail closed)
}

// NewHandler creates a Handler. ackStore and replayDLQ may be nil.
// When replayDLQ is nil the replay endpoint still works but DLQ fallback is
// disabled (Kafka errors will be surfaced as 503 instead of 202).
func NewHandler(svc ingestService, logger *slog.Logger, ackStore schemaACKStore) *Handler {
	return &Handler{svc: svc, logger: logger, ackStore: ackStore}
}

// WithReplayDLQ attaches a DLQ writer to the handler for replay fallback.
func (h *Handler) WithReplayDLQ(dlq application.ReplayDLQWriter) *Handler {
	h.replayDLQ = dlq
	return h
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

	c.Header("X-Recommended-Format", "msgpack")
	response.Created(c, IngestResponse{
		Accepted:  1,
		WrittenAt: time.Now().UTC(),
		ChannelID: channelID,
		RequestID: middleware.GetRequestID(c),
	})
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

	inputs, err := deserializeCompact(body, schema, d, channelID)
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

	// Record schema ACK after a successful compact-format ingest (fail-open: errors are logged only).
	if h.ackStore != nil {
		if err := h.ackStore.RecordACK(c.Request.Context(), channelID, schema.DeviceID, schema.SchemaVersion); err != nil {
			h.logger.WarnContext(c.Request.Context(), "schema ACK record failed", "error", err,
				"channel_id", channelID, "device_id", schema.DeviceID, "schema_version", schema.SchemaVersion)
		}
	}

	c.Header("X-Recommended-Format", "msgpack")
	response.Created(c, IngestResponse{
		Accepted:  len(inputs),
		WrittenAt: time.Now().UTC(),
		ChannelID: channelID,
		RequestID: middleware.GetRequestID(c),
	})
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
	if len(req.Readings) > maxBulkSize {
		response.Error(c, apierr.BadRequest(fmt.Sprintf("batch size exceeds maximum of %d", maxBulkSize)))
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

	c.Header("X-Recommended-Format", "msgpack")
	response.Created(c, IngestResponse{
		Accepted:  len(req.Readings),
		WrittenAt: time.Now().UTC(),
		ChannelID: channelID,
		RequestID: middleware.GetRequestID(c),
	})
}

// ThingSpeak godoc
// @Summary      ThingSpeak-compatible telemetry write endpoint
// @Tags         ingestion
// @Produce      plain
// @Param        api_key  query  string  true   "Device API key"
// @Param        field1   query  number  false  "Field 1 value"
// @Param        field2   query  number  false  "Field 2 value"
// @Param        field3   query  number  false  "Field 3 value"
// @Param        field4   query  number  false  "Field 4 value"
// @Param        field5   query  number  false  "Field 5 value"
// @Param        field6   query  number  false  "Field 6 value"
// @Param        field7   query  number  false  "Field 7 value"
// @Param        field8   query  number  false  "Field 8 value"
// @Success      200  {string}  string  "entry_id (Unix timestamp) on success, 0 on failure"
// @Router       /update [get]
func (h *Handler) ThingSpeak(lookup ChannelLookupFunc, logger *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		apiKey := c.Query("api_key")
		if apiKey == "" {
			apiKey = c.GetHeader("X-API-Key")
		}
		if apiKey == "" {
			c.String(http.StatusOK, "0")
			return
		}

		schema, err := lookup(c.Request.Context(), apiKey)
		if err != nil {
			if errors.Is(err, domain.ErrDeviceNotFound) {
				c.String(http.StatusOK, "0")
				return
			}
			logger.ErrorContext(c.Request.Context(), "thingspeak channel lookup failed", "error", err)
			c.String(http.StatusOK, "0")
			return
		}

		// Build a position→name map from the schema fields.
		posToName := make(map[int]string, len(schema.Fields))
		for _, f := range schema.Fields {
			posToName[int(f.Index)] = f.Name
		}

		// Parse field1..field8 query params and map to named fields.
		fields := make(map[string]float64)
		for i := 1; i <= 8; i++ {
			raw := c.Query(fmt.Sprintf("field%d", i))
			if raw == "" {
				continue
			}
			var val float64
			if _, err := fmt.Sscanf(raw, "%g", &val); err != nil {
				continue
			}
			name, ok := posToName[i]
			if !ok {
				continue
			}
			fields[name] = val
		}

		if len(fields) == 0 {
			c.String(http.StatusOK, "0")
			return
		}

		if err := h.svc.Ingest(c.Request.Context(), application.IngestInput{
			ChannelID: schema.ChannelID,
			DeviceID:  schema.DeviceID,
			Fields:    fields,
		}); err != nil {
			logger.ErrorContext(c.Request.Context(), "thingspeak ingest failed", "error", err)
			c.String(http.StatusOK, "0")
			return
		}

		entryID := time.Now().Unix()
		c.String(http.StatusOK, fmt.Sprintf("%d", entryID))
	}
}

const defaultReplayWindowDays = 30

// Replay godoc
// @Summary      Replay historical telemetry readings
// @Tags         ingestion
// @Accept       json
// @Produce      json
// @Param        channel_id  path      string        true  "Channel ID"
// @Param        request     body      ReplayRequest true  "Batch of historical readings"
// @Success      201         {object}  ReplayResponse
// @Success      202         {object}  ReplayResponse
// @Failure      400         {object}  map[string]interface{}
// @Failure      401         {object}  map[string]interface{}
// @Failure      403         {object}  map[string]interface{}
// @Security     ApiKeyAuth
// @Router       /v1/channels/{channel_id}/replay [post]
func (h *Handler) Replay(c *gin.Context) {
	schema, ok := contextDeviceSchema(c)
	if !ok {
		h.logger.ErrorContext(c.Request.Context(), "device_schema missing from auth context")
		response.Error(c, apierr.ErrInternalServerError)
		return
	}

	channelID := c.Param("channel_id")

	var req ReplayRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, apierr.BadRequest(err.Error()))
		return
	}
	if err := validator.Validate(&req); err != nil {
		response.ValidationError(c, err)
		return
	}

	inputs := make([]application.IngestInput, len(req.Readings))
	for i, r := range req.Readings {
		ts := r.Timestamp
		inputs[i] = application.IngestInput{
			ChannelID: channelID,
			DeviceID:  schema.DeviceID,
			Fields:    r.Fields,
			Timestamp: &ts,
		}
	}

	err := h.svc.IngestReplay(c.Request.Context(), inputs, defaultReplayWindowDays, h.replayDLQ)
	if err == nil {
		response.Created(c, ReplayResponse{
			Accepted:  len(req.Readings),
			ChannelID: channelID,
			RequestID: middleware.GetRequestID(c),
		})
		return
	}

	// If a DLQ is wired, readings were saved there — return 202 Accepted.
	if h.replayDLQ != nil {
		c.JSON(http.StatusAccepted, response.Envelope{
			Success: true,
			Data: ReplayResponse{
				Accepted:       0,
				QueuedForRetry: len(req.Readings),
				ChannelID:      channelID,
				RequestID:      middleware.GetRequestID(c),
			},
		})
		return
	}

	errorToHTTPResponse(c, err, h.logger)
}

// errorToHTTPResponse maps domain errors to appropriate HTTP responses.
func errorToHTTPResponse(c *gin.Context, err error, logger *slog.Logger) {
	var schemaMismatch *domain.SchemaMismatchError
	if errors.As(err, &schemaMismatch) {
		schemaURL := fmt.Sprintf("/v1/channels/%s/schema", schemaMismatch.ChannelID)
		c.JSON(http.StatusConflict, response.Envelope{
			Success: false,
			Error: &response.ErrBody{
				Code:    "schema_version_mismatch",
				Message: schemaMismatch.Error(),
				Details: gin.H{
					"current_version": schemaMismatch.CurrentVersion,
					"schema_url":      schemaURL,
				},
			},
		})
		return
	}

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
		errors.Is(err, domain.ErrTimestampFuture) ||
		errors.Is(err, domain.ErrTimestampOutOfReplayWindow)
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
