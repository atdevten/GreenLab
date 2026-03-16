package realtime

import (
	"sync"
	"time"

	"github.com/google/uuid"
)

// Subscription represents a WebSocket/SSE client subscription to a channel.
// It is transport-agnostic: the Send channel is the only communication path.
type Subscription struct {
	ID        uuid.UUID
	ChannelID string
	UserID    string
	Send      chan []byte
	CreatedAt time.Time
	mu       sync.Mutex
	isClosed bool
}

// NewSubscription creates a new Subscription.
func NewSubscription(channelID, userID string) *Subscription {
	return &Subscription{
		ID:        uuid.New(),
		ChannelID: channelID,
		UserID:    userID,
		Send:      make(chan []byte, 256),
		CreatedAt: time.Now().UTC(),
	}
}

// Write sends a message to the subscriber safely.
// It is a no-op if the subscription has already been closed.
func (s *Subscription) Write(msg []byte) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.isClosed {
		return
	}
	select {
	case s.Send <- msg:
	default:
		// Drop if buffer full — the subscriber is too slow.
	}
}

// Close permanently stops the subscription. Subsequent Write calls are
// silently ignored. Safe to call concurrently and more than once.
func (s *Subscription) Close() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.isClosed {
		return
	}
	s.isClosed = true
	close(s.Send)
}

// PushMessage is a real-time push payload.
type PushMessage struct {
	ChannelID string             `json:"channel_id"`
	DeviceID  string             `json:"device_id"`
	Fields    map[string]float64 `json:"fields"`
	Timestamp time.Time          `json:"timestamp"`
	Type      string             `json:"type"`
}
