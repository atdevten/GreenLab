package application

import (
	"encoding/json"
	"log/slog"
	"sync"

	"github.com/greenlab/query-realtime/internal/domain/realtime"
)

// Hub manages all active WebSocket/SSE subscriptions.
type Hub struct {
	mu            sync.RWMutex
	subscriptions map[string]map[string]*realtime.Subscription // channelID → subID → sub
	log           *slog.Logger
}

// NewHub creates a new Hub.
func NewHub(log *slog.Logger) *Hub {
	return &Hub{
		subscriptions: make(map[string]map[string]*realtime.Subscription),
		log:           log,
	}
}

// Subscribe registers a subscription.
func (h *Hub) Subscribe(sub *realtime.Subscription) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if _, ok := h.subscriptions[sub.ChannelID]; !ok {
		h.subscriptions[sub.ChannelID] = make(map[string]*realtime.Subscription)
	}
	h.subscriptions[sub.ChannelID][sub.ID.String()] = sub
	h.log.Info("client subscribed", "channel_id", sub.ChannelID, "sub_id", sub.ID.String())
}

// Unsubscribe removes a subscription.
func (h *Hub) Unsubscribe(sub *realtime.Subscription) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if subs, ok := h.subscriptions[sub.ChannelID]; ok {
		delete(subs, sub.ID.String())
		if len(subs) == 0 {
			delete(h.subscriptions, sub.ChannelID)
		}
	}
	h.log.Info("client unsubscribed", "sub_id", sub.ID.String())
}

// Broadcast sends a push message to all subscribers of a channel.
//
// The inner subscription map is snapshot into a slice under the read lock so
// that concurrent Subscribe/Unsubscribe calls cannot mutate the map while we
// iterate — which would cause a fatal runtime panic.
func (h *Hub) Broadcast(msg *realtime.PushMessage) {
	h.mu.RLock()
	inner, ok := h.subscriptions[msg.ChannelID]
	var targets []*realtime.Subscription
	if ok {
		targets = make([]*realtime.Subscription, 0, len(inner))
		for _, s := range inner {
			targets = append(targets, s)
		}
	}
	h.mu.RUnlock()

	if len(targets) == 0 {
		return
	}

	payload, err := json.Marshal(msg)
	if err != nil {
		h.log.Error("marshal push message", "error", err)
		return
	}

	for _, sub := range targets {
		sub.Write(payload)
	}
}

// SubscriberCount returns the number of active subscribers for a channel.
func (h *Hub) SubscriberCount(channelID string) int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.subscriptions[channelID])
}

// TotalSubscriptions returns the total number of active subscriptions.
func (h *Hub) TotalSubscriptions() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	total := 0
	for _, subs := range h.subscriptions {
		total += len(subs)
	}
	return total
}
