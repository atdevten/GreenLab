package tenant

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// ValidScopes holds the allowed scope values for workspace API keys.
var ValidScopes = map[string]bool{
	"read":  true,
	"write": true,
}

// WorkspaceAPIKey represents a workspace-scoped API key.
type WorkspaceAPIKey struct {
	ID          uuid.UUID
	WorkspaceID uuid.UUID
	Name        string
	Scope       string     // "read" or "write"
	KeyPrefix   string     // first 8 chars of raw key, for display
	KeyHash     string     // sha256 hash of the raw key
	CreatedAt   time.Time
	LastUsedAt  *time.Time
	RevokedAt   *time.Time
}

// NewWorkspaceAPIKey creates a new WorkspaceAPIKey, generates a wsk_-prefixed random key,
// stores the SHA256 hash, and returns the raw key only once.
func NewWorkspaceAPIKey(workspaceID uuid.UUID, name, scope string) (*WorkspaceAPIKey, string, error) {
	if name == "" {
		return nil, "", fmt.Errorf("NewWorkspaceAPIKey: %w", ErrInvalidName)
	}
	if !ValidScopes[scope] {
		return nil, "", fmt.Errorf("NewWorkspaceAPIKey: %w", ErrInvalidScope)
	}

	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return nil, "", fmt.Errorf("NewWorkspaceAPIKey.rand: %w", err)
	}
	plainKey := "wsk_" + hex.EncodeToString(raw) // 68-char key
	prefix := plainKey[:8]                        // "wsk_" + first 4 hex chars

	h := sha256.Sum256([]byte(plainKey))
	keyHash := hex.EncodeToString(h[:])

	now := time.Now().UTC()
	return &WorkspaceAPIKey{
		ID:          uuid.New(),
		WorkspaceID: workspaceID,
		Name:        name,
		Scope:       scope,
		KeyPrefix:   prefix,
		KeyHash:     keyHash,
		CreatedAt:   now,
	}, plainKey, nil
}
