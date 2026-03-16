package application

import (
	"context"
	"log/slog"

	"github.com/google/uuid"
	"github.com/greenlab/supporting/internal/domain/audit"
)

// ListTenantFilter holds optional filters for listing audit events by tenant.
type ListTenantFilter struct {
	ResourceType string
	Search       string
}

// EventRepository persists audit events.
type EventRepository interface {
	Append(ctx context.Context, event *audit.AuditEvent) error
	ListByTenant(ctx context.Context, tenantID string, filter ListTenantFilter, limit, offset int) ([]*audit.AuditEvent, int64, error)
	ListByResource(ctx context.Context, resourceType, resourceID string, limit, offset int) ([]*audit.AuditEvent, int64, error)
	GetByID(ctx context.Context, id uuid.UUID) (*audit.AuditEvent, error)
}

// AuditService handles audit event recording and querying.
type AuditService struct {
	repo   EventRepository
	logger *slog.Logger
}

func NewAuditService(repo EventRepository, logger *slog.Logger) *AuditService {
	return &AuditService{repo: repo, logger: logger}
}

// RecordInput holds raw fields for recording an audit event.
type RecordInput struct {
	TenantID     string
	UserID       string
	EventType    string
	ResourceID   string
	ResourceType string
	IPAddress    string
	UserAgent    string
	Payload      []byte
}

// Record appends a new audit event (append-only, never updates or deletes).
func (s *AuditService) Record(ctx context.Context, in RecordInput) (*audit.AuditEvent, error) {
	event := audit.NewAuditEvent(
		in.TenantID, in.UserID, in.EventType,
		in.ResourceID, in.ResourceType,
		in.IPAddress, in.UserAgent, in.Payload,
	)
	if err := s.repo.Append(ctx, event); err != nil {
		return nil, err
	}
	return event, nil
}

// ListByTenant retrieves audit events for a tenant with pagination and optional filters.
func (s *AuditService) ListByTenant(ctx context.Context, tenantID string, filter ListTenantFilter, limit, offset int) ([]*audit.AuditEvent, int64, error) {
	return s.repo.ListByTenant(ctx, tenantID, filter, limit, offset)
}

// ListByResource retrieves audit events for a specific resource.
func (s *AuditService) ListByResource(ctx context.Context, resourceType, resourceID string, limit, offset int) ([]*audit.AuditEvent, int64, error) {
	return s.repo.ListByResource(ctx, resourceType, resourceID, limit, offset)
}

// GetEvent retrieves a single audit event by ID.
func (s *AuditService) GetEvent(ctx context.Context, id string) (*audit.AuditEvent, error) {
	uid, err := uuid.Parse(id)
	if err != nil {
		return nil, audit.ErrInvalidEventID
	}
	return s.repo.GetByID(ctx, uid)
}
