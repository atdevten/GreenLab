package http

import "time"

// Video stream DTOs
type CreateStreamRequest struct {
	DeviceID    string `json:"device_id"    validate:"required"`
	WorkspaceID string `json:"workspace_id" validate:"required"`
	Name        string `json:"name"         validate:"required"`
	Description string `json:"description"`
	Protocol    string `json:"protocol"     validate:"required"`
	SourceURL   string `json:"source_url"`
}

type UpdateStreamStatusRequest struct {
	Status string `json:"status" validate:"required"`
}

// StreamResponse omits StorageKey intentionally — it is an internal S3 key
// and is never exposed directly; clients use the presigned URL endpoints instead.
type StreamResponse struct {
	ID           string    `json:"id"`
	DeviceID     string    `json:"device_id"`
	WorkspaceID  string    `json:"workspace_id"`
	Name         string    `json:"name"`
	Description  string    `json:"description"`
	Protocol     string    `json:"protocol"`
	SourceURL    string    `json:"source_url"`
	Status       string    `json:"status"`
	ThumbnailURL string    `json:"thumbnail_url"`
	DurationSec  int       `json:"duration_sec"`
	CreatedAt    time.Time `json:"created_at"`
}

type PresignedURLResponse struct {
	URL       string    `json:"url"`
	ExpiresAt time.Time `json:"expires_at"`
}

// Audit event DTOs
type AuditEventResponse struct {
	ID           string    `json:"id"`
	TenantID     string    `json:"tenant_id"`
	UserID       string    `json:"user_id"`
	UserName     string    `json:"user_name"`
	EventType    string    `json:"event_type"`
	Action       string    `json:"action"`        // alias for event_type — frontend-expected field
	ResourceID   string    `json:"resource_id"`
	ResourceType string    `json:"resource_type"`
	Target       string    `json:"target"`        // alias for resource_id — frontend-expected field
	IPAddress    string    `json:"ip_address"`
	IP           string    `json:"ip"`            // alias for ip_address — frontend-expected field
	UserAgent    string    `json:"user_agent"`
	Payload      []byte    `json:"payload,omitempty" swaggertype:"string" format:"base64"`
	CreatedAt    time.Time `json:"created_at"`
}
