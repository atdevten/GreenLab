package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/lib/pq"
	"github.com/greenlab/iam/internal/domain/tenant"
)

type apiKeyRow struct {
	ID        string         `db:"id"`
	TenantID  string         `db:"tenant_id"`
	UserID    string         `db:"user_id"`
	Name      string         `db:"name"`
	KeyPrefix string         `db:"key_prefix"`
	KeyHash   string         `db:"key_hash"`
	Scopes    pq.StringArray `db:"scopes"`
	CreatedAt time.Time      `db:"created_at"`
	LastUsed  *time.Time     `db:"last_used"`
}

func (r *apiKeyRow) toAPIKey() tenant.APIKey {
	return tenant.APIKey{
		ID:        r.ID,
		TenantID:  r.TenantID,
		UserID:    r.UserID,
		Name:      r.Name,
		KeyPrefix: r.KeyPrefix,
		KeyHash:   r.KeyHash,
		Scopes:    []string(r.Scopes),
		CreatedAt: r.CreatedAt,
		LastUsed:  r.LastUsed,
	}
}

type APIKeyRepo struct{ db *sqlx.DB }

func NewAPIKeyRepo(db *sqlx.DB) *APIKeyRepo { return &APIKeyRepo{db: db} }

func (r *APIKeyRepo) ListAPIKeys(ctx context.Context, tenantID string) ([]tenant.APIKey, error) {
	var rows []apiKeyRow
	err := r.db.SelectContext(ctx, &rows, `
		SELECT id, tenant_id, user_id, name, key_prefix, key_hash, scopes, created_at, last_used
		FROM api_keys WHERE tenant_id=$1 ORDER BY created_at DESC`, tenantID)
	if err != nil {
		return nil, fmt.Errorf("APIKeyRepo.ListAPIKeys: %w", err)
	}
	keys := make([]tenant.APIKey, len(rows))
	for i, row := range rows {
		keys[i] = row.toAPIKey()
	}
	return keys, nil
}

func (r *APIKeyRepo) CreateAPIKey(ctx context.Context, key tenant.APIKey) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO api_keys (id, tenant_id, user_id, name, key_prefix, key_hash, scopes, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
		key.ID, key.TenantID, key.UserID, key.Name,
		key.KeyPrefix, key.KeyHash, pq.StringArray(key.Scopes), key.CreatedAt)
	if err != nil {
		return fmt.Errorf("APIKeyRepo.CreateAPIKey: %w", err)
	}
	return nil
}

func (r *APIKeyRepo) DeleteAPIKey(ctx context.Context, id, tenantID string) error {
	res, err := r.db.ExecContext(ctx, `
		DELETE FROM api_keys WHERE id=$1 AND tenant_id=$2`, id, tenantID)
	if err != nil {
		return fmt.Errorf("APIKeyRepo.DeleteAPIKey: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("APIKeyRepo.DeleteAPIKey: rows affected: %w", err)
	}
	if n == 0 {
		return fmt.Errorf("APIKeyRepo.DeleteAPIKey: %w", tenant.ErrAPIKeyNotFound)
	}
	return nil
}

// GetByHash is used internally by the ingestion service for API key validation.
func (r *APIKeyRepo) GetByHash(ctx context.Context, keyHash string) (*tenant.APIKey, error) {
	var row apiKeyRow
	err := r.db.GetContext(ctx, &row, `
		SELECT id, tenant_id, user_id, name, key_prefix, key_hash, scopes, created_at, last_used
		FROM api_keys WHERE key_hash=$1`, keyHash)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("APIKeyRepo.GetByHash: %w", tenant.ErrAPIKeyNotFound)
	}
	if err != nil {
		return nil, fmt.Errorf("APIKeyRepo.GetByHash: %w", err)
	}
	k := row.toAPIKey()
	return &k, nil
}
