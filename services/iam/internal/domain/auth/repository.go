package auth

import (
	"context"

	"github.com/google/uuid"
)

type UserRepository interface {
	Create(ctx context.Context, user *User) error
	GetByID(ctx context.Context, id uuid.UUID) (*User, error)
	GetByEmail(ctx context.Context, email string) (*User, error)
	GetByVerifyToken(ctx context.Context, token string) (*User, error)
	GetByResetToken(ctx context.Context, token string) (*User, error)
	Update(ctx context.Context, user *User) error
	Delete(ctx context.Context, id uuid.UUID) error
	GetRoles(ctx context.Context, userID uuid.UUID) ([]Role, error)
	SetRoles(ctx context.Context, userID uuid.UUID, roles []Role) error
}

type TokenRepository interface {
	SaveRefreshToken(ctx context.Context, token *RefreshToken) error
	GetRefreshToken(ctx context.Context, tokenHash string) (*RefreshToken, error)
	// GetAndRevokeRefreshToken atomically marks the token as revoked and returns it.
	// Returns ErrTokenNotFound if the token does not exist, is already revoked, or is expired.
	GetAndRevokeRefreshToken(ctx context.Context, tokenHash string) (*RefreshToken, error)
	RevokeRefreshToken(ctx context.Context, tokenHash string) error
	RevokeFamily(ctx context.Context, family uuid.UUID) error
	RevokeAllForUser(ctx context.Context, userID uuid.UUID) error
	DeleteExpired(ctx context.Context) error
}

type CacheRepository interface {
	SetUserSession(ctx context.Context, userID string, data interface{}, ttlSeconds int) error
	GetUserSession(ctx context.Context, userID string) ([]byte, error)
	DeleteUserSession(ctx context.Context, userID string) error
	BlacklistToken(ctx context.Context, jti string, ttlSeconds int) error
	IsTokenBlacklisted(ctx context.Context, jti string) (bool, error)
}

type EventPublisher interface {
	PublishUserRegistered(ctx context.Context, user *User) error
	PublishUserLoggedIn(ctx context.Context, user *User, ip string) error
	PublishPasswordChanged(ctx context.Context, userID uuid.UUID) error
	PublishEmailVerified(ctx context.Context, userID uuid.UUID) error
}
