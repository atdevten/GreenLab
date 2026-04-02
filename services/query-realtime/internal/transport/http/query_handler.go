package http

import (
	"bytes"
	"context"
	"encoding/csv"
	"errors"
	"fmt"
	"log/slog"
	"sort"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/greenlab/query-realtime/internal/domain/query"
	"github.com/greenlab/shared/pkg/apierr"
	"github.com/greenlab/shared/pkg/response"
	"github.com/greenlab/shared/pkg/validator"
)

// queryService is the local interface the QueryHandler depends on.
// Using query.QueryRequest directly avoids duplicating the field set in a
// separate application-layer input type.
type queryService interface {
	Query(ctx context.Context, req query.QueryRequest) (*query.QueryResult, error)
	QueryLatest(ctx context.Context, channelID, fieldName string) (*query.LatestReading, error)
}

// QueryHandler handles HTTP query requests.
type QueryHandler struct {
	svc    queryService
	logger *slog.Logger
}

// NewQueryHandler creates a new QueryHandler.
func NewQueryHandler(svc queryService, logger *slog.Logger) *QueryHandler {
	return &QueryHandler{svc: svc, logger: logger}
}

// Health godoc
// @Summary      Health check
// @Tags         health
// @Produce      json
// @Success      200  {object}  map[string]interface{}
// @Router       /health [get]
func (h *QueryHandler) Health(c *gin.Context) {
	response.OK(c, gin.H{"status": "ok"})
}

// Query godoc
// @Summary      Query time-series data for a channel
// @Tags         query
// @Produce      json
// @Param        channel_id  query     string  true   "Channel ID (UUID)"
// @Param        field       query     string  false  "Field name to filter"
// @Param        start       query     string  false  "Start time (RFC3339)"
// @Param        end         query     string  false  "End time (RFC3339)"
// @Param        limit       query     int     false  "Max number of points"
// @Param        aggregate   query     string  false  "Aggregation function (mean, max, min, sum)"
// @Param        window      query     string  false  "Aggregation window (e.g. 1m, 5m, 1h)"
// @Success      200         {object}  QueryResponse
// @Failure      400         {object}  map[string]interface{}
// @Failure      401         {object}  map[string]interface{}
// @Security     BearerAuth
// @Router       /api/v1/query [get]
func (h *QueryHandler) Query(c *gin.Context) {
	var params QueryParams
	if err := c.ShouldBindQuery(&params); err != nil {
		response.Error(c, apierr.BadRequest(err.Error()))
		return
	}
	if err := validator.Validate(&params); err != nil {
		response.ValidationError(c, err)
		return
	}

	req := query.QueryRequest{
		ChannelID: params.ChannelID,
		FieldName: params.FieldName,
		Limit:     params.Limit,
		Aggregate: params.Aggregate,
		Window:    params.Window,
	}

	if params.Start != "" {
		t, err := time.Parse(time.RFC3339, params.Start)
		if err != nil {
			response.Error(c, apierr.BadRequest("start must be RFC3339 (e.g. 2024-01-01T00:00:00Z)"))
			return
		}
		req.Start = t
	}
	if params.End != "" {
		t, err := time.Parse(time.RFC3339, params.End)
		if err != nil {
			response.Error(c, apierr.BadRequest("end must be RFC3339 (e.g. 2024-01-01T00:00:00Z)"))
			return
		}
		req.End = t
	}

	result, err := h.svc.Query(c.Request.Context(), req)
	if err != nil {
		if isQueryValidationError(err) {
			response.Error(c, apierr.BadRequest(err.Error()))
			return
		}
		h.logger.Error("query failed", "channel_id", params.ChannelID, "error", err)
		response.Error(c, apierr.ErrInternalServerError)
		return
	}

	if c.Query("format") == "csv" {
		csvBytes, err := buildQueryCSV(result)
		if err != nil {
			h.logger.Error("csv build failed", "channel_id", params.ChannelID, "error", err)
			response.Error(c, apierr.ErrInternalServerError)
			return
		}
		c.Header("Content-Disposition", "attachment; filename=\"query-export.csv\"")
		c.Data(200, "text/csv", csvBytes)
		return
	}

	response.OK(c, toQueryResponse(result))
}

// QueryLatest godoc
// @Summary      Get the latest reading for a channel field
// @Tags         query
// @Produce      json
// @Param        channel_id  query     string  true   "Channel ID (UUID)"
// @Param        field       query     string  false  "Field name"
// @Success      200         {object}  LatestReadingResponse
// @Failure      400         {object}  map[string]interface{}
// @Failure      404         {object}  map[string]interface{}
// @Security     BearerAuth
// @Router       /api/v1/query/latest [get]
func (h *QueryHandler) QueryLatest(c *gin.Context) {
	channelID := c.Query("channel_id")
	if channelID == "" {
		response.Error(c, apierr.BadRequest("channel_id is required"))
		return
	}
	if _, err := uuid.Parse(channelID); err != nil {
		response.Error(c, apierr.BadRequest("channel_id must be a valid UUID"))
		return
	}

	fieldName := c.Query("field")

	latest, err := h.svc.QueryLatest(c.Request.Context(), channelID, fieldName)
	if err != nil {
		if errors.Is(err, query.ErrNoDataFound) {
			response.Error(c, apierr.NotFound("reading"))
			return
		}
		h.logger.Error("query latest failed", "channel_id", channelID, "error", err)
		response.Error(c, apierr.ErrInternalServerError)
		return
	}

	response.OK(c, toLatestResponse(latest))
}

func buildQueryCSV(r *query.QueryResult) ([]byte, error) {
	// Collect unique field names while preserving first-occurrence order of timestamps.
	fieldSet := make(map[string]struct{})
	for _, dp := range r.DataPoints {
		fieldSet[dp.Field] = struct{}{}
	}
	fields := make([]string, 0, len(fieldSet))
	for f := range fieldSet {
		fields = append(fields, f)
	}
	sort.Strings(fields)

	// Group data points by timestamp, preserving first-occurrence order.
	type rowKey = string // RFC3339 timestamp string
	rowOrder := make([]rowKey, 0)
	rowData := make(map[rowKey]map[string]float64)
	for _, dp := range r.DataPoints {
		ts := dp.Timestamp.UTC().Format(time.RFC3339)
		if _, exists := rowData[ts]; !exists {
			rowOrder = append(rowOrder, ts)
			rowData[ts] = make(map[string]float64)
		}
		rowData[ts][dp.Field] = dp.Value
	}

	var buf bytes.Buffer
	w := csv.NewWriter(&buf)

	header := append([]string{"timestamp"}, fields...)
	if err := w.Write(header); err != nil {
		return nil, fmt.Errorf("buildQueryCSV header: %w", err)
	}

	for _, ts := range rowOrder {
		row := make([]string, 1+len(fields))
		row[0] = ts
		for i, f := range fields {
			if v, ok := rowData[ts][f]; ok {
				row[i+1] = fmt.Sprintf("%g", v)
			}
		}
		if err := w.Write(row); err != nil {
			return nil, fmt.Errorf("buildQueryCSV row: %w", err)
		}
	}

	w.Flush()
	if err := w.Error(); err != nil {
		return nil, fmt.Errorf("buildQueryCSV flush: %w", err)
	}
	return buf.Bytes(), nil
}

// isQueryValidationError reports whether err is a domain-level validation
// error that should map to 400 Bad Request.
func isQueryValidationError(err error) bool {
	return errors.Is(err, query.ErrInvalidChannelID) ||
		errors.Is(err, query.ErrInvalidTimeRange) ||
		errors.Is(err, query.ErrInvalidFieldName) ||
		errors.Is(err, query.ErrInvalidAggregate) ||
		errors.Is(err, query.ErrInvalidWindow)
}

func toQueryResponse(r *query.QueryResult) *QueryResponse {
	dps := make([]DataPointResponse, len(r.DataPoints))
	for i, dp := range r.DataPoints {
		dps[i] = DataPointResponse{Timestamp: dp.Timestamp, Value: dp.Value, Field: dp.Field}
	}
	return &QueryResponse{
		ChannelID:  r.ChannelID,
		FieldName:  r.FieldName,
		DataPoints: dps,
		Count:      r.Count,
		Start:      r.Start,
		End:        r.End,
	}
}

func toLatestResponse(lr *query.LatestReading) *LatestReadingResponse {
	return &LatestReadingResponse{
		ChannelID: lr.ChannelID,
		FieldName: lr.FieldName,
		Value:     lr.Value,
		Timestamp: lr.Timestamp,
	}
}
