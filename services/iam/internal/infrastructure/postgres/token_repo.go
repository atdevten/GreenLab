package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/greenlab/iam/internal/domain/auth"
)

type refreshTokenRow struct {
	ID        uuid.UUID `db:"id"`
	UserID    uuid.UUID `db:"user_id"`
	TokenHash string    `db:"token_hash"`
	Family    uuid.UUID `db:"family"`
	ExpiresAt time.Time `db:"expires_at"`
	CreatedAt time.Time `db:"created_at"`
	Revoked   bool      `db:"revoked"`
	UserAgent string    `db:"user_agent"`
	IPAddress string    `db:"ip_address"`
}

func (r refreshTokenRow) toToken() *auth.RefreshToken {
	return &auth.RefreshToken{
		ID:        r.ID,
		UserID:    r.UserID,
		TokenHash: r.TokenHash,
		Family:    r.Family,
		ExpiresAt: r.ExpiresAt,
		CreatedAt: r.CreatedAt,
		Revoked:   r.Revoked,
		UserAgent: r.UserAgent,
		IPAddress: r.IPAddress,
	}
}

type TokenRepo struct{ db *sqlx.DB }

func NewTokenRepo(db *sqlx.DB) *TokenRepo { return &TokenRepo{db: db} }

func (r *TokenRepo) SaveRefreshToken(ctx context.Context, t *auth.RefreshToken) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO refresh_tokens (id, user_id, token_hash, family, expires_at, user_agent, ip_address, revoked)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
		t.ID, t.UserID, t.TokenHash, t.Family, t.ExpiresAt, t.UserAgent, t.IPAddress, t.Revoked,
	)
	return err
}

func (r *TokenRepo) GetRefreshToken(ctx context.Context, tokenHash string) (*auth.RefreshToken, error) {
	var row refreshTokenRow
	err := r.db.GetContext(ctx, &row,
		`SELECT id, user_id, token_hash, family, expires_at, created_at, revoked, user_agent, ip_address
		 FROM refresh_tokens WHERE token_hash = $1`, tokenHash)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("TokenRepo.GetRefreshToken: %w", auth.ErrTokenNotFound)
	}
	if err != nil {
		return nil, fmt.Errorf("TokenRepo.GetRefreshToken: %w", err)
	}
	return row.toToken(), nil
}

func (r *TokenRepo) GetAndRevokeRefreshToken(ctx context.Context, tokenHash string) (*auth.RefreshToken, error) {
	var row refreshTokenRow
	err := r.db.GetContext(ctx, &row, `
		UPDATE refresh_tokens
		SET revoked = TRUE
		WHERE token_hash = $1
		  AND revoked = FALSE
		  AND expires_at > NOW()
		RETURNING id, user_id, token_hash, family, expires_at, created_at, revoked, user_agent, ip_address`,
		tokenHash)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("TokenRepo.GetAndRevokeRefreshToken: %w", auth.ErrTokenNotFound)
	}
	if err != nil {
		return nil, fmt.Errorf("TokenRepo.GetAndRevokeRefreshToken: %w", err)
	}
	return row.toToken(), nil
}

func (r *TokenRepo) RevokeRefreshToken(ctx context.Context, tokenHash string) error {
	_, err := r.db.ExecContext(ctx, `UPDATE refresh_tokens SET revoked = TRUE WHERE token_hash = $1`, tokenHash)
	return err
}

func (r *TokenRepo) RevokeFamily(ctx context.Context, family uuid.UUID) error {
	_, err := r.db.ExecContext(ctx, `UPDATE refresh_tokens SET revoked = TRUE WHERE family = $1`, family)
	return err
}

func (r *TokenRepo) RevokeAllForUser(ctx context.Context, userID uuid.UUID) error {
	_, err := r.db.ExecContext(ctx, `UPDATE refresh_tokens SET revoked = TRUE WHERE user_id = $1`, userID)
	return err
}

func (r *TokenRepo) DeleteExpired(ctx context.Context) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM refresh_tokens WHERE expires_at < NOW()`)
	return err
}
