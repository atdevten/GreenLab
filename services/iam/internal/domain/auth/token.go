package auth

import (
	"time"

	"github.com/google/uuid"
)

type TokenType string

const (
	TokenTypeAccess  TokenType = "access"
	TokenTypeRefresh TokenType = "refresh"
)

type RefreshToken struct {
	ID        uuid.UUID
	UserID    uuid.UUID
	TokenHash string
	Family    uuid.UUID
	ExpiresAt time.Time
	CreatedAt time.Time
	Revoked   bool
	UserAgent string
	IPAddress string
}

func NewRefreshToken(userID uuid.UUID, tokenHash string, family uuid.UUID, ttl time.Duration, userAgent, ip string) *RefreshToken {
	now := time.Now().UTC()
	return &RefreshToken{
		ID:        uuid.New(),
		UserID:    userID,
		TokenHash: tokenHash,
		Family:    family,
		ExpiresAt: now.Add(ttl),
		CreatedAt: now,
		Revoked:   false,
		UserAgent: userAgent,
		IPAddress: ip,
	}
}

func (t *RefreshToken) IsExpired() bool { return time.Now().UTC().After(t.ExpiresAt) }

func (t *RefreshToken) IsValid() bool { return !t.Revoked && !t.IsExpired() }

type TokenPair struct {
	AccessToken  string
	RefreshToken string
	ExpiresAt    time.Time
	TokenType    string
}
