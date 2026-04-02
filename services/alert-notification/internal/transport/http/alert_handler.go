package http

import (
	"context"
	"errors"
	"log/slog"

	"github.com/gin-gonic/gin"
	"github.com/greenlab/alert-notification/internal/application"
	"github.com/greenlab/alert-notification/internal/domain/alert"
	"github.com/greenlab/alert-notification/internal/domain/delivery"
	"github.com/greenlab/shared/pkg/apierr"
	"github.com/greenlab/shared/pkg/pagination"
	"github.com/greenlab/shared/pkg/response"
	"github.com/greenlab/shared/pkg/validator"
)

// alertService is the local interface AlertHandler depends on.
type alertService interface {
	CreateRule(ctx context.Context, in application.CreateRuleInput) (*alert.Rule, error)
	GetRule(ctx context.Context, id string) (*alert.Rule, error)
	ListRules(ctx context.Context, workspaceID string, limit, offset int) ([]*alert.Rule, int64, error)
	UpdateRule(ctx context.Context, id string, in application.UpdateRuleInput) (*alert.Rule, error)
	DeleteRule(ctx context.Context, id string) error
	ListDeliveries(ctx context.Context, ruleID string, limit, offset int) ([]*delivery.Log, int64, error)
	VerifyWebhookSignature(ctx context.Context, id, payload, signature string) (bool, error)
}

// AlertHandler handles HTTP requests for alert rules.
type AlertHandler struct {
	svc    alertService
	logger *slog.Logger
}

// NewAlertHandler creates a new AlertHandler.
func NewAlertHandler(svc alertService, logger *slog.Logger) *AlertHandler {
	return &AlertHandler{svc: svc, logger: logger}
}

// Health godoc
// @Summary      Health check
// @Tags         health
// @Produce      json
// @Success      200  {object}  map[string]interface{}
// @Router       /health [get]
func (h *AlertHandler) Health(c *gin.Context) {
	response.OK(c, gin.H{"status": "ok"})
}

// CreateRule godoc
// @Summary      Create a new alert rule
// @Tags         alert-rules
// @Accept       json
// @Produce      json
// @Param        request  body      CreateRuleRequest  true  "Alert rule definition"
// @Success      201      {object}  RuleResponse
// @Failure      400      {object}  map[string]interface{}
// @Security     BearerAuth
// @Router       /api/v1/alert-rules [post]
func (h *AlertHandler) CreateRule(c *gin.Context) {
	var req CreateRuleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, apierr.BadRequest(err.Error()))
		return
	}
	if err := validator.Validate(&req); err != nil {
		response.ValidationError(c, err)
		return
	}
	rule, err := h.svc.CreateRule(c.Request.Context(), application.CreateRuleInput{
		ChannelID: req.ChannelID, WorkspaceID: req.WorkspaceID,
		Name: req.Name, FieldName: req.FieldName,
		Condition: req.Condition, Threshold: *req.Threshold,
		Severity: req.Severity, Message: req.Message, CooldownSec: req.CooldownSec,
		WebhookSecret: req.Secret,
	})
	if err != nil {
		if isAlertValidationError(err) {
			response.Error(c, apierr.BadRequest(err.Error()))
			return
		}
		h.logger.Error("create rule failed", "error", err)
		response.Error(c, apierr.ErrInternalServerError)
		return
	}
	response.Created(c, toRuleResponse(rule))
}

// GetRule godoc
// @Summary      Get an alert rule by ID
// @Tags         alert-rules
// @Produce      json
// @Param        id  path      string  true  "Rule ID"
// @Success      200  {object}  RuleResponse
// @Failure      404  {object}  map[string]interface{}
// @Security     BearerAuth
// @Router       /api/v1/alert-rules/{id} [get]
func (h *AlertHandler) GetRule(c *gin.Context) {
	rule, err := h.svc.GetRule(c.Request.Context(), c.Param("id"))
	if err != nil {
		if errors.Is(err, alert.ErrRuleNotFound) {
			response.Error(c, apierr.NotFound("rule"))
			return
		}
		if errors.Is(err, alert.ErrInvalidRuleID) {
			response.Error(c, apierr.BadRequest(err.Error()))
			return
		}
		h.logger.Error("get rule failed", "id", c.Param("id"), "error", err)
		response.Error(c, apierr.ErrInternalServerError)
		return
	}
	response.OK(c, toRuleResponse(rule))
}

// ListRules godoc
// @Summary      List alert rules for a workspace
// @Tags         alert-rules
// @Produce      json
// @Param        workspace_id  query     string  false  "Filter by workspace ID"
// @Param        limit         query     int     false  "Page size"
// @Param        offset        query     int     false  "Page offset"
// @Success      200           {array}   RuleResponse
// @Failure      400           {object}  map[string]interface{}
// @Security     BearerAuth
// @Router       /api/v1/alert-rules [get]
func (h *AlertHandler) ListRules(c *gin.Context) {
	page := pagination.ParseOffset(c)
	rules, total, err := h.svc.ListRules(c.Request.Context(), c.Query("workspace_id"), page.Limit, page.Offset())
	if err != nil {
		if isAlertValidationError(err) {
			response.Error(c, apierr.BadRequest(err.Error()))
			return
		}
		h.logger.Error("list rules failed", "error", err)
		response.Error(c, apierr.ErrInternalServerError)
		return
	}
	items := make([]*RuleResponse, len(rules))
	for i, r := range rules {
		items[i] = toRuleResponse(r)
	}
	response.OKWithMeta(c, items, pagination.NewOffsetResult(items, total, page))
}

// UpdateRule godoc
// @Summary      Update an alert rule
// @Tags         alert-rules
// @Accept       json
// @Produce      json
// @Param        id       path      string            true  "Rule ID"
// @Param        request  body      UpdateRuleRequest  true  "Update fields"
// @Success      200      {object}  RuleResponse
// @Failure      400      {object}  map[string]interface{}
// @Failure      404      {object}  map[string]interface{}
// @Security     BearerAuth
// @Router       /api/v1/alert-rules/{id} [put]
func (h *AlertHandler) UpdateRule(c *gin.Context) {
	var req UpdateRuleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, apierr.BadRequest(err.Error()))
		return
	}
	rule, err := h.svc.UpdateRule(c.Request.Context(), c.Param("id"), application.UpdateRuleInput{
		Name: req.Name, Threshold: req.Threshold, Severity: req.Severity,
		Message: req.Message, Enabled: req.Enabled, CooldownSec: req.CooldownSec,
		WebhookSecret: req.Secret,
	})
	if err != nil {
		if errors.Is(err, alert.ErrRuleNotFound) {
			response.Error(c, apierr.NotFound("rule"))
			return
		}
		h.logger.Error("update rule failed", "id", c.Param("id"), "error", err)
		response.Error(c, apierr.ErrInternalServerError)
		return
	}
	response.OK(c, toRuleResponse(rule))
}

// DeleteRule godoc
// @Summary      Delete an alert rule
// @Tags         alert-rules
// @Param        id  path  string  true  "Rule ID"
// @Success      204  "No Content"
// @Failure      404  {object}  map[string]interface{}
// @Security     BearerAuth
// @Router       /api/v1/alert-rules/{id} [delete]
func (h *AlertHandler) DeleteRule(c *gin.Context) {
	if err := h.svc.DeleteRule(c.Request.Context(), c.Param("id")); err != nil {
		if errors.Is(err, alert.ErrRuleNotFound) {
			response.Error(c, apierr.NotFound("rule"))
			return
		}
		if errors.Is(err, alert.ErrInvalidRuleID) {
			response.Error(c, apierr.BadRequest(err.Error()))
			return
		}
		h.logger.Error("delete rule failed", "id", c.Param("id"), "error", err)
		response.Error(c, apierr.ErrInternalServerError)
		return
	}
	response.NoContent(c)
}

// ListDeliveries godoc
// @Summary      List webhook delivery logs for an alert rule
// @Tags         alert-rules
// @Produce      json
// @Param        id      path      string  true   "Rule ID"
// @Param        limit   query     int     false  "Page size"
// @Param        offset  query     int     false  "Page offset"
// @Success      200     {array}   DeliveryLogResponse
// @Failure      400     {object}  map[string]interface{}
// @Failure      404     {object}  map[string]interface{}
// @Security     BearerAuth
// @Router       /api/v1/alert-rules/{id}/deliveries [get]
func (h *AlertHandler) ListDeliveries(c *gin.Context) {
	page := pagination.ParseOffset(c)
	logs, total, err := h.svc.ListDeliveries(c.Request.Context(), c.Param("id"), page.Limit, page.Offset())
	if err != nil {
		if errors.Is(err, alert.ErrInvalidRuleID) {
			response.Error(c, apierr.BadRequest(err.Error()))
			return
		}
		h.logger.Error("list deliveries failed", "rule_id", c.Param("id"), "error", err)
		response.Error(c, apierr.ErrInternalServerError)
		return
	}
	items := make([]*DeliveryLogResponse, len(logs))
	for i, l := range logs {
		items[i] = toDeliveryLogResponse(l)
	}
	response.OKWithMeta(c, items, pagination.NewOffsetResult(items, total, page))
}

// VerifySignature godoc
// @Summary      Sandbox endpoint to verify an HMAC-SHA256 webhook signature
// @Description  Computes HMAC-SHA256(secret, payload) for the rule and reports whether the provided signature matches.
// @Tags         alert-rules
// @Accept       json
// @Produce      json
// @Param        id       path      string                    true  "Rule ID"
// @Param        request  body      VerifySignatureRequest    true  "Payload and signature to verify"
// @Success      200      {object}  VerifySignatureResponse
// @Failure      400      {object}  map[string]interface{}
// @Failure      404      {object}  map[string]interface{}
// @Security     BearerAuth
// @Router       /api/v1/alert-rules/{id}/verify-signature [post]
func (h *AlertHandler) VerifySignature(c *gin.Context) {
	var req VerifySignatureRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, apierr.BadRequest(err.Error()))
		return
	}
	if err := validator.Validate(&req); err != nil {
		response.ValidationError(c, err)
		return
	}
	valid, err := h.svc.VerifyWebhookSignature(c.Request.Context(), c.Param("id"), req.Payload, req.Signature)
	if err != nil {
		if errors.Is(err, alert.ErrRuleNotFound) {
			response.Error(c, apierr.NotFound("rule"))
			return
		}
		if errors.Is(err, alert.ErrInvalidRuleID) {
			response.Error(c, apierr.BadRequest(err.Error()))
			return
		}
		if errors.Is(err, alert.ErrNoWebhookSecret) {
			response.Error(c, apierr.BadRequest(err.Error()))
			return
		}
		h.logger.Error("verify signature failed", "id", c.Param("id"), "error", err)
		response.Error(c, apierr.ErrInternalServerError)
		return
	}
	response.OK(c, VerifySignatureResponse{Valid: valid})
}

// isAlertValidationError reports whether err is a domain-level input error
// that should map to 400 Bad Request.
func isAlertValidationError(err error) bool {
	return errors.Is(err, alert.ErrInvalidChannelID) ||
		errors.Is(err, alert.ErrInvalidWorkspace) ||
		errors.Is(err, alert.ErrInvalidRuleID)
}

func toDeliveryLogResponse(l *delivery.Log) *DeliveryLogResponse {
	return &DeliveryLogResponse{
		ID:           l.ID.String(),
		RuleID:       l.RuleID.String(),
		URL:          l.URL,
		HTTPStatus:   l.HTTPStatus,
		LatencyMS:    l.LatencyMS,
		ResponseBody: l.ResponseBody,
		ErrorMsg:     l.ErrorMsg,
		DeliveredAt:  l.DeliveredAt,
	}
}

func toRuleResponse(r *alert.Rule) *RuleResponse {
	return &RuleResponse{
		ID:          r.ID.String(),
		ChannelID:   r.ChannelID.String(),
		WorkspaceID: r.WorkspaceID.String(),
		Name:        r.Name,
		FieldName:   r.FieldName,
		Condition:   string(r.Condition),
		Threshold:   r.Threshold,
		Severity:    string(r.Severity),
		Message:     r.Message,
		Enabled:     r.Enabled,
		CooldownSec: r.CooldownSec,
		CreatedAt:   r.CreatedAt,
	}
}
