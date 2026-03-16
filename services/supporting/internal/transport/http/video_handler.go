package http

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/greenlab/shared/pkg/apierr"
	"github.com/greenlab/shared/pkg/pagination"
	"github.com/greenlab/shared/pkg/response"
	"github.com/greenlab/shared/pkg/validator"
	"github.com/greenlab/supporting/internal/application"
	"github.com/greenlab/supporting/internal/domain/video"
)

// videoService is the local interface VideoHandler depends on.
type videoService interface {
	CreateStream(ctx context.Context, in application.CreateStreamInput) (*video.Stream, error)
	GetStream(ctx context.Context, id string) (*video.Stream, error)
	ListStreams(ctx context.Context, deviceID string, limit, offset int) ([]*video.Stream, int64, error)
	UpdateStreamStatus(ctx context.Context, id string, status video.StreamStatus) (*video.Stream, error)
	GetUploadURL(ctx context.Context, streamID, contentType string) (string, error)
	GetDownloadURL(ctx context.Context, streamID string) (string, error)
	DeleteStream(ctx context.Context, id string) error
}

// VideoHandler handles HTTP requests for video streams.
type VideoHandler struct {
	svc    videoService
	logger *slog.Logger
}

func NewVideoHandler(svc videoService, logger *slog.Logger) *VideoHandler {
	return &VideoHandler{svc: svc, logger: logger}
}

// Health godoc
// @Summary      Health check
// @Tags         health
// @Produce      json
// @Success      200  {object}  map[string]interface{}
// @Router       /health [get]
func (h *VideoHandler) Health(c *gin.Context) {
	response.OK(c, gin.H{"status": "ok"})
}

// CreateStream godoc
// @Summary      Register a new video stream
// @Tags         streams
// @Accept       json
// @Produce      json
// @Param        request  body      CreateStreamRequest  true  "Stream details"
// @Success      201      {object}  StreamResponse
// @Failure      400      {object}  map[string]interface{}
// @Security     BearerAuth
// @Router       /api/v1/streams [post]
func (h *VideoHandler) CreateStream(c *gin.Context) {
	var req CreateStreamRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, apierr.BadRequest(err.Error()))
		return
	}
	if err := validator.Validate(&req); err != nil {
		response.ValidationError(c, err)
		return
	}
	s, err := h.svc.CreateStream(c.Request.Context(), application.CreateStreamInput{
		DeviceID: req.DeviceID, WorkspaceID: req.WorkspaceID,
		Name: req.Name, Description: req.Description,
		Protocol: req.Protocol, SourceURL: req.SourceURL,
	})
	if err != nil {
		if isVideoValidationError(err) {
			response.Error(c, apierr.BadRequest(err.Error()))
			return
		}
		h.logger.Error("create stream failed", "error", err)
		response.Error(c, apierr.ErrInternalServerError)
		return
	}
	response.Created(c, toStreamResponse(s))
}

// GetStream godoc
// @Summary      Get a video stream by ID
// @Tags         streams
// @Produce      json
// @Param        id  path      string  true  "Stream ID"
// @Success      200  {object}  StreamResponse
// @Failure      404  {object}  map[string]interface{}
// @Security     BearerAuth
// @Router       /api/v1/streams/{id} [get]
func (h *VideoHandler) GetStream(c *gin.Context) {
	s, err := h.svc.GetStream(c.Request.Context(), c.Param("id"))
	if err != nil {
		if errors.Is(err, video.ErrStreamNotFound) {
			response.Error(c, apierr.NotFound("stream"))
			return
		}
		if errors.Is(err, video.ErrInvalidStreamID) {
			response.Error(c, apierr.BadRequest(err.Error()))
			return
		}
		h.logger.Error("get stream failed", "id", c.Param("id"), "error", err)
		response.Error(c, apierr.ErrInternalServerError)
		return
	}
	response.OK(c, toStreamResponse(s))
}

// ListStreams godoc
// @Summary      List video streams for a device
// @Tags         streams
// @Produce      json
// @Param        device_id  query     string  false  "Filter by device ID"
// @Param        limit      query     int     false  "Page size"
// @Param        offset     query     int     false  "Page offset"
// @Success      200        {array}   StreamResponse
// @Failure      400        {object}  map[string]interface{}
// @Security     BearerAuth
// @Router       /api/v1/streams [get]
func (h *VideoHandler) ListStreams(c *gin.Context) {
	page := pagination.ParseOffset(c)
	streams, total, err := h.svc.ListStreams(c.Request.Context(), c.Query("device_id"), page.Limit, page.Offset())
	if err != nil {
		if errors.Is(err, video.ErrInvalidDeviceID) {
			response.Error(c, apierr.BadRequest(err.Error()))
			return
		}
		h.logger.Error("list streams failed", "error", err)
		response.Error(c, apierr.ErrInternalServerError)
		return
	}
	items := make([]*StreamResponse, len(streams))
	for i, s := range streams {
		items[i] = toStreamResponse(s)
	}
	response.OKWithMeta(c, items, pagination.NewOffsetResult(items, total, page))
}

// UpdateStreamStatus godoc
// @Summary      Update the status of a video stream
// @Tags         streams
// @Accept       json
// @Produce      json
// @Param        id       path      string                    true  "Stream ID"
// @Param        request  body      UpdateStreamStatusRequest  true  "New status"
// @Success      200      {object}  StreamResponse
// @Failure      400      {object}  map[string]interface{}
// @Failure      404      {object}  map[string]interface{}
// @Security     BearerAuth
// @Router       /api/v1/streams/{id}/status [patch]
func (h *VideoHandler) UpdateStreamStatus(c *gin.Context) {
	var req UpdateStreamStatusRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, apierr.BadRequest(err.Error()))
		return
	}
	if err := validator.Validate(&req); err != nil {
		response.ValidationError(c, err)
		return
	}
	s, err := h.svc.UpdateStreamStatus(c.Request.Context(), c.Param("id"), video.StreamStatus(req.Status))
	if err != nil {
		if errors.Is(err, video.ErrStreamNotFound) {
			response.Error(c, apierr.NotFound("stream"))
			return
		}
		if errors.Is(err, video.ErrInvalidStreamID) || errors.Is(err, video.ErrInvalidStatus) {
			response.Error(c, apierr.BadRequest(err.Error()))
			return
		}
		h.logger.Error("update stream status failed", "id", c.Param("id"), "error", err)
		response.Error(c, apierr.ErrInternalServerError)
		return
	}
	response.OK(c, toStreamResponse(s))
}

// GetUploadURL godoc
// @Summary      Get a pre-signed upload URL for a stream recording
// @Tags         streams
// @Produce      json
// @Param        id            path      string  true   "Stream ID"
// @Param        content_type  query     string  false  "Content type (default: video/mp4)"
// @Success      200           {object}  PresignedURLResponse
// @Failure      404           {object}  map[string]interface{}
// @Security     BearerAuth
// @Router       /api/v1/streams/{id}/upload-url [get]
func (h *VideoHandler) GetUploadURL(c *gin.Context) {
	contentType := c.DefaultQuery("content_type", "video/mp4")
	url, err := h.svc.GetUploadURL(c.Request.Context(), c.Param("id"), contentType)
	if err != nil {
		if errors.Is(err, video.ErrStreamNotFound) {
			response.Error(c, apierr.NotFound("stream"))
			return
		}
		if errors.Is(err, video.ErrInvalidStreamID) {
			response.Error(c, apierr.BadRequest(err.Error()))
			return
		}
		h.logger.Error("get upload url failed", "id", c.Param("id"), "error", err)
		response.Error(c, apierr.ErrInternalServerError)
		return
	}
	response.OK(c, PresignedURLResponse{URL: url, ExpiresAt: time.Now().Add(15 * time.Minute)})
}

// GetDownloadURL godoc
// @Summary      Get a pre-signed download URL for a stream recording
// @Tags         streams
// @Produce      json
// @Param        id  path      string  true  "Stream ID"
// @Success      200  {object}  PresignedURLResponse
// @Failure      404  {object}  map[string]interface{}
// @Security     BearerAuth
// @Router       /api/v1/streams/{id}/download-url [get]
func (h *VideoHandler) GetDownloadURL(c *gin.Context) {
	url, err := h.svc.GetDownloadURL(c.Request.Context(), c.Param("id"))
	if err != nil {
		if errors.Is(err, video.ErrStreamNotFound) || errors.Is(err, video.ErrNoRecording) {
			response.Error(c, apierr.NotFound("stream recording"))
			return
		}
		if errors.Is(err, video.ErrInvalidStreamID) {
			response.Error(c, apierr.BadRequest(err.Error()))
			return
		}
		h.logger.Error("get download url failed", "id", c.Param("id"), "error", err)
		response.Error(c, apierr.ErrInternalServerError)
		return
	}
	response.OK(c, PresignedURLResponse{URL: url, ExpiresAt: time.Now().Add(1 * time.Hour)})
}

// DeleteStream godoc
// @Summary      Delete a video stream
// @Tags         streams
// @Param        id  path  string  true  "Stream ID"
// @Success      204  "No Content"
// @Failure      404  {object}  map[string]interface{}
// @Security     BearerAuth
// @Router       /api/v1/streams/{id} [delete]
func (h *VideoHandler) DeleteStream(c *gin.Context) {
	if err := h.svc.DeleteStream(c.Request.Context(), c.Param("id")); err != nil {
		if errors.Is(err, video.ErrStreamNotFound) {
			response.Error(c, apierr.NotFound("stream"))
			return
		}
		if errors.Is(err, video.ErrInvalidStreamID) {
			response.Error(c, apierr.BadRequest(err.Error()))
			return
		}
		h.logger.Error("delete stream failed", "id", c.Param("id"), "error", err)
		response.Error(c, apierr.ErrInternalServerError)
		return
	}
	response.NoContent(c)
}

// isVideoValidationError reports whether err is a domain input validation error.
func isVideoValidationError(err error) bool {
	return errors.Is(err, video.ErrInvalidDeviceID) ||
		errors.Is(err, video.ErrInvalidWorkspaceID) ||
		errors.Is(err, video.ErrInvalidStreamID) ||
		errors.Is(err, video.ErrInvalidProtocol) ||
		errors.Is(err, video.ErrInvalidStatus)
}

func toStreamResponse(s *video.Stream) *StreamResponse {
	return &StreamResponse{
		ID: s.ID.String(), DeviceID: s.DeviceID.String(), WorkspaceID: s.WorkspaceID.String(),
		Name: s.Name, Description: s.Description, Protocol: string(s.Protocol),
		SourceURL: s.SourceURL, Status: string(s.Status),
		ThumbnailURL: s.ThumbnailURL, DurationSec: s.DurationSec,
		CreatedAt: s.CreatedAt,
	}
}
