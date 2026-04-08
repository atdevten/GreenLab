package audit

import (
	"time"

	"github.com/google/uuid"
)

// AuditEvent is an immutable append-only record of a system event.
type AuditEvent struct {
	ID           uuid.UUID
	TenantID     string
	UserID       string
	UserName     string // human-readable name or email, for display
	EventType    string
	ResourceID   string
	ResourceType string
	IPAddress    string
	UserAgent    string
	Payload      []byte // JSONB raw event payload
	CreatedAt    time.Time
}

// NewAuditEvent constructs a new immutable AuditEvent.
func NewAuditEvent(tenantID, userID, userName, eventType, resourceID, resourceType, ipAddress, userAgent string, payload []byte) *AuditEvent {
	return &AuditEvent{
		ID:           uuid.New(),
		TenantID:     tenantID,
		UserID:       userID,
		UserName:     userName,
		EventType:    eventType,
		ResourceID:   resourceID,
		ResourceType: resourceType,
		IPAddress:    ipAddress,
		UserAgent:    userAgent,
		Payload:      payload,
		CreatedAt:    time.Now().UTC(),
	}
}
