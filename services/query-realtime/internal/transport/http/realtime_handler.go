package http

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"sync"
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

// wsCommand is a client-to-server control message.
type wsCommand struct {
	Action    string `json:"action"`     // "subscribe" | "unsubscribe"
	ChannelID string `json:"channel_id"` // target channel
}

// WebSocket godoc
// @Summary      Subscribe to real-time readings via WebSocket (multiplexed)
// @Tags         realtime
// @Produce      json
// @Param        channel_id  query  string  false  "Channel ID (legacy: auto-subscribes on connect)"
// @Success      101  "Switching Protocols — upgrades to WebSocket connection"
// @Failure      400  {object}  map[string]interface{}
// @Failure      401  {object}  map[string]interface{}
// @Security     BearerAuth
// @Router       /api/v1/ws [get]
func (h *RealtimeHandler) WebSocket(c *gin.Context) {
	userID, _ := sharedMiddleware.GetUserID(c)

	conn, err := h.upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		h.logger.Error("ws upgrade failed", "error", err)
		return
	}

	// connSend is the single outbound channel for this connection.
	connSend := make(chan []byte, 512)

	// subs holds per-channel subscriptions for this connection.
	var (
		subsMu sync.Mutex
		subs   = make(map[string]*realtime.Subscription)
		wg     sync.WaitGroup
	)

	// writeDone is closed once the write pump exits so the read pump can wait.
	writeDone := make(chan struct{})

	// subscribe creates a subscription for channelID and starts a forwarder
	// goroutine. It is a no-op if already subscribed.
	subscribe := func(channelID string) {
		subsMu.Lock()
		defer subsMu.Unlock()
		if _, exists := subs[channelID]; exists {
			return
		}
		sub := realtime.NewSubscription(channelID, userID)
		h.hub.Subscribe(sub)
		subs[channelID] = sub

		wg.Add(1)
		go func() {
			defer wg.Done()
			for msg := range sub.Send {
				select {
				case connSend <- msg:
				default:
					// Drop message if outbound buffer is full — client is too slow.
				}
			}
		}()
	}

	// unsubscribe removes and cleans up a subscription by channelID.
	unsubscribe := func(channelID string) {
		subsMu.Lock()
		defer subsMu.Unlock()
		sub, exists := subs[channelID]
		if !exists {
			return
		}
		h.hub.Unsubscribe(sub)
		sub.Close()
		delete(subs, channelID)
	}

	// Backward-compat: if a channel_id query param is present, auto-subscribe.
	if legacyChannelID := c.Query("channel_id"); legacyChannelID != "" {
		subscribe(legacyChannelID)
	}

	// Write pump: drains connSend and sends frames to the WebSocket.
	go func() {
		defer close(writeDone)
		ctx := c.Request.Context()
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				conn.WriteMessage(websocket.CloseMessage,
					websocket.FormatCloseMessage(websocket.CloseGoingAway, ""))
				return
			case msg, ok := <-connSend:
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

	// Read pump: parses JSON commands and drives the pong handler /
	// client-disconnect detection.
	conn.SetReadLimit(1024)
	conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	conn.SetPongHandler(func(string) error {
		conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		return nil
	})

	for {
		_, raw, err := conn.ReadMessage()
		if err != nil {
			break
		}
		var cmd wsCommand
		if jsonErr := json.Unmarshal(raw, &cmd); jsonErr != nil || cmd.ChannelID == "" {
			continue
		}
		switch cmd.Action {
		case "subscribe":
			subscribe(cmd.ChannelID)
		case "unsubscribe":
			unsubscribe(cmd.ChannelID)
		}
	}

	// Cleanup: unsubscribe and close all active subs, then wait for forwarders
	// to drain, close connSend, and finally wait for the write pump to exit.
	subsMu.Lock()
	for channelID, sub := range subs {
		h.hub.Unsubscribe(sub)
		sub.Close()
		delete(subs, channelID)
	}
	subsMu.Unlock()

	wg.Wait()
	close(connSend)
	<-writeDone
	conn.Close()
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
