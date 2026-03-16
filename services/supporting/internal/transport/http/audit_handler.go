package http

import (
	"bytes"
	"context"
	"encoding/csv"
	"errors"
	"fmt"
	"log/slog"

	"github.com/gin-gonic/gin"
	"github.com/greenlab/shared/pkg/apierr"
	"github.com/greenlab/shared/pkg/middleware"
	"github.com/greenlab/shared/pkg/pagination"
	"github.com/greenlab/shared/pkg/response"
	"github.com/greenlab/supporting/internal/application"
	"github.com/greenlab/supporting/internal/domain/audit"
)

// auditService is the local interface AuditHandler depends on.
type auditService interface {
	GetEvent(ctx context.Context, id string) (*audit.AuditEvent, error)
	ListByTenant(ctx context.Context, tenantID string, filter application.ListTenantFilter, limit, offset int) ([]*audit.AuditEvent, int64, error)
	ListByResource(ctx context.Context, resourceType, resourceID string, limit, offset int) ([]*audit.AuditEvent, int64, error)
}

// AuditHandler handles HTTP requests for audit events.
type AuditHandler struct {
	svc    auditService
	logger *slog.Logger
}

func NewAuditHandler(svc auditService, logger *slog.Logger) *AuditHandler {
	return &AuditHandler{svc: svc, logger: logger}
}

// GetEvent godoc
// @Summary      Get an audit event by ID
// @Tags         audit
// @Produce      json
// @Param        id  path      string  true  "Audit Event ID"
// @Success      200  {object}  AuditEventResponse
// @Failure      404  {object}  map[string]interface{}
// @Security     BearerAuth
// @Router       /api/v1/audit/events/{id} [get]
func (h *AuditHandler) GetEvent(c *gin.Context) {
	event, err := h.svc.GetEvent(c.Request.Context(), c.Param("id"))
	if err != nil {
		if errors.Is(err, audit.ErrEventNotFound) {
			response.Error(c, apierr.NotFound("audit event"))
			return
		}
		if errors.Is(err, audit.ErrInvalidEventID) {
			response.Error(c, apierr.BadRequest(err.Error()))
			return
		}
		h.logger.Error("get audit event failed", "id", c.Param("id"), "error", err)
		response.Error(c, apierr.ErrInternalServerError)
		return
	}
	response.OK(c, toAuditEventResponse(event))
}

// ListByTenant godoc
// @Summary      List audit events for the authenticated tenant
// @Tags         audit
// @Produce      json
// @Param        resource_type  query     string  false  "Filter by resource type"
// @Param        search         query     string  false  "Search in user_name, action, target (case-insensitive)"
// @Param        format         query     string  false  "Set to 'csv' for CSV export"
// @Param        limit          query     int     false  "Page size"
// @Param        offset         query     int     false  "Page offset"
// @Success      200            {array}   AuditEventResponse
// @Failure      401            {object}  map[string]interface{}
// @Security     BearerAuth
// @Router       /api/v1/audit/events [get]
func (h *AuditHandler) ListByTenant(c *gin.Context) {
	tenantID, ok := middleware.GetTenantID(c)
	if !ok {
		response.Error(c, apierr.Unauthorized("missing tenant"))
		return
	}
	page := pagination.ParseOffset(c)
	filter := application.ListTenantFilter{
		ResourceType: c.Query("resource_type"),
		Search:       c.Query("search"),
	}
	events, total, err := h.svc.ListByTenant(c.Request.Context(), tenantID, filter, page.Limit, page.Offset())
	if err != nil {
		h.logger.Error("list audit events by tenant failed", "tenant_id", tenantID, "error", err)
		response.Error(c, apierr.ErrInternalServerError)
		return
	}

	if c.Query("format") == "csv" {
		csvBytes, err := buildAuditCSV(events)
		if err != nil {
			h.logger.Error("audit csv build failed", "tenant_id", tenantID, "error", err)
			response.Error(c, apierr.ErrInternalServerError)
			return
		}
		c.Header("Content-Disposition", "attachment; filename=\"audit-log.csv\"")
		c.Data(200, "text/csv", csvBytes)
		return
	}

	items := make([]*AuditEventResponse, len(events))
	for i, e := range events {
		items[i] = toAuditEventResponse(e)
	}
	response.OKWithMeta(c, items, pagination.NewOffsetResult(items, total, page))
}

func buildAuditCSV(events []*audit.AuditEvent) ([]byte, error) {
	var buf bytes.Buffer
	w := csv.NewWriter(&buf)
	header := []string{"id", "user_id", "event_type", "resource_type", "resource_id", "ip_address", "created_at"}
	if err := w.Write(header); err != nil {
		return nil, fmt.Errorf("buildAuditCSV header: %w", err)
	}
	for _, e := range events {
		row := []string{
			e.ID.String(),
			e.UserID,
			e.EventType,
			e.ResourceType,
			e.ResourceID,
			e.IPAddress,
			e.CreatedAt.UTC().Format("2006-01-02T15:04:05Z"),
		}
		if err := w.Write(row); err != nil {
			return nil, fmt.Errorf("buildAuditCSV row: %w", err)
		}
	}
	w.Flush()
	if err := w.Error(); err != nil {
		return nil, fmt.Errorf("buildAuditCSV flush: %w", err)
	}
	return buf.Bytes(), nil
}

// ListByResource godoc
// @Summary      List audit events for a specific resource
// @Tags         audit
// @Produce      json
// @Param        resource_type  query     string  true   "Resource type (e.g. device, channel)"
// @Param        resource_id    query     string  true   "Resource ID"
// @Param        limit          query     int     false  "Page size"
// @Param        offset         query     int     false  "Page offset"
// @Success      200            {array}   AuditEventResponse
// @Failure      400            {object}  map[string]interface{}
// @Security     BearerAuth
// @Router       /api/v1/audit/events/resource [get]
func (h *AuditHandler) ListByResource(c *gin.Context) {
	page := pagination.ParseOffset(c)
	resourceType := c.Query("resource_type")
	resourceID := c.Query("resource_id")
	if resourceType == "" || resourceID == "" {
		response.Error(c, apierr.BadRequest("resource_type and resource_id are required"))
		return
	}
	events, total, err := h.svc.ListByResource(c.Request.Context(), resourceType, resourceID, page.Limit, page.Offset())
	if err != nil {
		h.logger.Error("list audit events by resource failed", "resource_type", resourceType, "resource_id", resourceID, "error", err)
		response.Error(c, apierr.ErrInternalServerError)
		return
	}
	items := make([]*AuditEventResponse, len(events))
	for i, e := range events {
		items[i] = toAuditEventResponse(e)
	}
	response.OKWithMeta(c, items, pagination.NewOffsetResult(items, total, page))
}

func toAuditEventResponse(e *audit.AuditEvent) *AuditEventResponse {
	return &AuditEventResponse{
		ID:           e.ID.String(),
		TenantID:     e.TenantID,
		UserID:       e.UserID,
		EventType:    e.EventType,
		ResourceID:   e.ResourceID,
		ResourceType: e.ResourceType,
		IPAddress:    e.IPAddress,
		UserAgent:    e.UserAgent,
		Payload:      e.Payload,
		CreatedAt:    e.CreatedAt,
	}
}
