package http

import (
	"io"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/greenlab/query-realtime/internal/domain/realtime"
	"github.com/greenlab/shared/pkg/apierr"
	sharedMiddleware "github.com/greenlab/shared/pkg/middleware"
	"github.com/greenlab/shared/pkg/response"
)

// allowedOrigins returns the set of permitted WebSocket origins.
// ALLOWED_ORIGINS is a comma-separated list of origins, e.g.
// "https://app.example.com,https://dash.example.com".
// When empty the service falls back to the FRONTEND_URL env var.
// An explicit wildcard "*" re-enables allow-all (dev only).
func allowedOrigins() map[string]struct{} {
	raw := os.Getenv("ALLOWED_ORIGINS")
	if raw == "" {
		raw = os.Getenv("FRONTEND_URL")
	}
	set := make(map[string]struct{})
	for _, o := range strings.Split(raw, ",") {
		o = strings.TrimSpace(o)
		if o != "" {
			set[o] = struct{}{}
		}
	}
	return set
}

// realtimeHub is the local interface the RealtimeHandler depends on.
type realtimeHub interface {
	Subscribe(sub *realtime.Subscription)
	Unsubscribe(sub *realtime.Subscription)
	TotalSubscriptions() int
}

// RealtimeHandler handles WebSocket and SSE connections.
type RealtimeHandler struct {
	hub      realtimeHub
	logger   *slog.Logger
	upgrader websocket.Upgrader
}

// NewRealtimeHandler creates a new RealtimeHandler.
// allowedOrigins() is called once here so env-var parsing does not happen
// on every WebSocket upgrade request.
func NewRealtimeHandler(hub realtimeHub, logger *slog.Logger) *RealtimeHandler {
	origins := allowedOrigins()
	h := &RealtimeHandler{hub: hub, logger: logger}
	h.upgrader = websocket.Upgrader{
		ReadBufferSize:  1024,
		WriteBufferSize: 4096,
		CheckOrigin: func(r *http.Request) bool {
			origin := r.Header.Get("Origin")
			// Same-origin requests (e.g. server-side clients) have no Origin header.
			if origin == "" {
				return true
			}
			// Wildcard allows all origins — acceptable in local dev when set explicitly.
			if _, ok := origins["*"]; ok {
				return true
			}
			// If no origins are configured, reject cross-origin connections by default.
			if len(origins) == 0 {
				return false
			}
			_, ok := origins[origin]
			return ok
		},
	}
	return h
}

// WebSocket godoc
// @Summary      Subscribe to real-time readings via WebSocket
// @Tags         realtime
// @Produce      json
// @Param        channel_id  query  string  true  "Channel ID to subscribe to"
// @Success      101  "Switching Protocols — upgrades to WebSocket connection"
// @Failure      400  {object}  map[string]interface{}
// @Failure      401  {object}  map[string]interface{}
// @Security     BearerAuth
// @Router       /api/v1/ws [get]
func (h *RealtimeHandler) WebSocket(c *gin.Context) {
	channelID := c.Query("channel_id")
	if channelID == "" {
		response.Error(c, apierr.BadRequest("channel_id is required"))
		return
	}

	userID, _ := sharedMiddleware.GetUserID(c)

	conn, err := h.upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		h.logger.Error("ws upgrade failed", "error", err)
		return
	}

	sub := realtime.NewSubscription(channelID, userID)
	h.hub.Subscribe(sub)
	defer func() {
		h.hub.Unsubscribe(sub)
		sub.Close() // signal write pump to exit and drain the channel
		conn.Close()
	}()

	// Write pump: exits on context cancellation (request gone), channel close
	// (subscription closed by defer), or a write error.
	// Capturing the context here prevents the goroutine from leaking after the
	// handler returns — previously, with no context.Done case, the goroutine
	// would block on <-sub.Send indefinitely.
	go func() {
		ctx := c.Request.Context()
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				conn.WriteMessage(websocket.CloseMessage,
					websocket.FormatCloseMessage(websocket.CloseGoingAway, ""))
				return
			case msg, ok := <-sub.Send:
				conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
				if !ok {
					conn.WriteMessage(websocket.CloseMessage, []byte{})
					return
				}
				if err := conn.WriteMessage(websocket.TextMessage, msg); err != nil {
					h.logger.Debug("ws write error", "error", err)
					return
				}
			case <-ticker.C:
				conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
				if err := conn.WriteMessage(websocket.PingMessage, nil); err != nil {
					return
				}
			}
		}
	}()

	// Read pump — drives the pong handler and detects client disconnect.
	conn.SetReadLimit(512)
	conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	conn.SetPongHandler(func(string) error {
		conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		return nil
	})
	for {
		if _, _, err := conn.ReadMessage(); err != nil {
			break
		}
	}
}

// SSE godoc
// @Summary      Subscribe to real-time readings via Server-Sent Events
// @Tags         realtime
// @Produce      text/event-stream
// @Param        channel_id  query  string  true  "Channel ID to subscribe to"
// @Success      200  "Streams SSE events with Content-Type: text/event-stream"
// @Failure      400  {object}  map[string]interface{}
// @Failure      401  {object}  map[string]interface{}
// @Security     BearerAuth
// @Router       /api/v1/sse [get]
func (h *RealtimeHandler) SSE(c *gin.Context) {
	channelID := c.Query("channel_id")
	if channelID == "" {
		response.Error(c, apierr.BadRequest("channel_id is required"))
		return
	}

	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("X-Accel-Buffering", "no")

	userID, _ := sharedMiddleware.GetUserID(c)
	sub := realtime.NewSubscription(channelID, userID)
	h.hub.Subscribe(sub)
	defer func() {
		h.hub.Unsubscribe(sub)
		sub.Close() // prevent any concurrent Write call from blocking after we return
	}()

	notify := c.Request.Context().Done()
	c.Stream(func(w io.Writer) bool {
		select {
		case <-notify:
			return false
		case msg, ok := <-sub.Send:
			if !ok {
				return false
			}
			c.SSEvent("reading", string(msg))
			return true
		}
	})
}

// Stats godoc
// @Summary      Get real-time hub statistics
// @Tags         realtime
// @Produce      json
// @Success      200  {object}  map[string]interface{}
// @Security     BearerAuth
// @Router       /api/v1/stats [get]
func (h *RealtimeHandler) Stats(c *gin.Context) {
	response.OK(c, gin.H{
		"total_subscriptions": h.hub.TotalSubscriptions(),
	})
}
