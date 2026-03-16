package http

import (
	"context"
	"errors"
	"log/slog"

	"github.com/gin-gonic/gin"
	"github.com/greenlab/alert-notification/internal/application"
	"github.com/greenlab/alert-notification/internal/domain/notification"
	"github.com/greenlab/shared/pkg/apierr"
	"github.com/greenlab/shared/pkg/middleware"
	"github.com/greenlab/shared/pkg/pagination"
	"github.com/greenlab/shared/pkg/response"
	"github.com/greenlab/shared/pkg/validator"
)

// notificationService is the local interface NotificationHandler depends on.
type notificationService interface {
	Send(ctx context.Context, in application.SendNotificationInput) (*notification.Notification, error)
	GetNotification(ctx context.Context, id string) (*notification.Notification, error)
	ListNotifications(ctx context.Context, workspaceID string, limit, offset int) ([]*notification.Notification, int64, error)
	MarkNotificationRead(ctx context.Context, id, tenantID string) error
	MarkAllNotificationsRead(ctx context.Context, tenantID string) error
}

// NotificationHandler handles HTTP requests for notifications.
type NotificationHandler struct {
	svc    notificationService
	logger *slog.Logger
}

// NewNotificationHandler creates a new NotificationHandler.
func NewNotificationHandler(svc notificationService, logger *slog.Logger) *NotificationHandler {
	return &NotificationHandler{svc: svc, logger: logger}
}

// SendNotification godoc
// @Summary      Send a notification
// @Tags         notifications
// @Accept       json
// @Produce      json
// @Param        request  body      SendNotificationRequest  true  "Notification details"
// @Success      201      {object}  NotificationResponse
// @Failure      400      {object}  map[string]interface{}
// @Security     BearerAuth
// @Router       /api/v1/notifications [post]
func (h *NotificationHandler) SendNotification(c *gin.Context) {
	var req SendNotificationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, apierr.BadRequest(err.Error()))
		return
	}
	if err := validator.Validate(&req); err != nil {
		response.ValidationError(c, err)
		return
	}
	n, err := h.svc.Send(c.Request.Context(), application.SendNotificationInput{
		WorkspaceID: req.WorkspaceID, ChannelType: req.ChannelType,
		Recipient: req.Recipient, Subject: req.Subject, Body: req.Body,
	})
	if err != nil {
		if errors.Is(err, notification.ErrInvalidWorkspace) {
			response.Error(c, apierr.BadRequest(err.Error()))
			return
		}
		h.logger.Error("send notification failed", "error", err)
		response.Error(c, apierr.ErrInternalServerError)
		return
	}
	response.Created(c, toNotificationResponse(n))
}

// GetNotification godoc
// @Summary      Get a notification by ID
// @Tags         notifications
// @Produce      json
// @Param        id  path      string  true  "Notification ID"
// @Success      200  {object}  NotificationResponse
// @Failure      404  {object}  map[string]interface{}
// @Security     BearerAuth
// @Router       /api/v1/notifications/{id} [get]
func (h *NotificationHandler) GetNotification(c *gin.Context) {
	n, err := h.svc.GetNotification(c.Request.Context(), c.Param("id"))
	if err != nil {
		if errors.Is(err, notification.ErrNotificationNotFound) {
			response.Error(c, apierr.NotFound("notification"))
			return
		}
		if errors.Is(err, notification.ErrInvalidNotificationID) {
			response.Error(c, apierr.BadRequest(err.Error()))
			return
		}
		h.logger.Error("get notification failed", "id", c.Param("id"), "error", err)
		response.Error(c, apierr.ErrInternalServerError)
		return
	}
	response.OK(c, toNotificationResponse(n))
}

// ListNotifications godoc
// @Summary      List notifications for a workspace
// @Tags         notifications
// @Produce      json
// @Param        workspace_id  query     string  false  "Filter by workspace ID"
// @Param        limit         query     int     false  "Page size"
// @Param        offset        query     int     false  "Page offset"
// @Success      200           {array}   NotificationResponse
// @Failure      400           {object}  map[string]interface{}
// @Security     BearerAuth
// @Router       /api/v1/notifications [get]
func (h *NotificationHandler) ListNotifications(c *gin.Context) {
	page := pagination.ParseOffset(c)
	notifications, total, err := h.svc.ListNotifications(c.Request.Context(), c.Query("workspace_id"), page.Limit, page.Offset())
	if err != nil {
		if errors.Is(err, notification.ErrInvalidWorkspace) {
			response.Error(c, apierr.BadRequest(err.Error()))
			return
		}
		h.logger.Error("list notifications failed", "error", err)
		response.Error(c, apierr.ErrInternalServerError)
		return
	}
	items := make([]*NotificationResponse, len(notifications))
	for i, n := range notifications {
		items[i] = toNotificationResponse(n)
	}
	response.OKWithMeta(c, items, pagination.NewOffsetResult(items, total, page))
}

// MarkRead godoc
// @Summary      Mark a notification as read
// @Tags         notifications
// @Produce      json
// @Param        id  path      string  true  "Notification ID"
// @Success      200  {object}  map[string]interface{}
// @Failure      404  {object}  map[string]interface{}
// @Security     BearerAuth
// @Router       /api/v1/notifications/{id}/read [patch]
func (h *NotificationHandler) MarkRead(c *gin.Context) {
	tenantID, ok := middleware.GetTenantID(c)
	if !ok {
		response.Error(c, apierr.Unauthorized("missing tenant"))
		return
	}
	if err := h.svc.MarkNotificationRead(c.Request.Context(), c.Param("id"), tenantID); err != nil {
		if errors.Is(err, notification.ErrNotificationNotFound) {
			response.Error(c, apierr.NotFound("notification"))
			return
		}
		h.logger.Error("mark notification read failed", "id", c.Param("id"), "error", err)
		response.Error(c, apierr.ErrInternalServerError)
		return
	}
	response.OK(c, gin.H{"id": c.Param("id"), "read": true})
}

// MarkAllRead godoc
// @Summary      Mark all notifications as read for the authenticated tenant
// @Tags         notifications
// @Success      204  "No Content"
// @Failure      500  {object}  map[string]interface{}
// @Security     BearerAuth
// @Router       /api/v1/notifications/read-all [post]
func (h *NotificationHandler) MarkAllRead(c *gin.Context) {
	tenantID, ok := middleware.GetTenantID(c)
	if !ok {
		response.Error(c, apierr.Unauthorized("missing tenant"))
		return
	}
	if err := h.svc.MarkAllNotificationsRead(c.Request.Context(), tenantID); err != nil {
		h.logger.Error("mark all notifications read failed", "tenant_id", tenantID, "error", err)
		response.Error(c, apierr.ErrInternalServerError)
		return
	}
	response.NoContent(c)
}

func toNotificationResponse(n *notification.Notification) *NotificationResponse {
	return &NotificationResponse{
		ID:          n.ID.String(),
		WorkspaceID: n.WorkspaceID.String(),
		ChannelType: string(n.ChannelType),
		Recipient:   n.Recipient,
		Subject:     n.Subject,
		Status:      string(n.Status),
		Retries:     n.Retries,
		SentAt:      n.SentAt,
		Read:        n.Read,
		ReadAt:      n.ReadAt,
		CreatedAt:   n.CreatedAt,
	}
}
