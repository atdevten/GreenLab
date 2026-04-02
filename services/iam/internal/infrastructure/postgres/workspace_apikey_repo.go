package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/greenlab/iam/internal/domain/tenant"
)

type workspaceAPIKeyRow struct {
	ID          uuid.UUID  `db:"id"`
	WorkspaceID uuid.UUID  `db:"workspace_id"`
	Name        string     `db:"name"`
	Scope       string     `db:"scope"`
	KeyPrefix   string     `db:"key_prefix"`
	KeyHash     string     `db:"key_hash"`
	CreatedAt   time.Time  `db:"created_at"`
	LastUsedAt  *time.Time `db:"last_used_at"`
	RevokedAt   *time.Time `db:"revoked_at"`
}

func (r workspaceAPIKeyRow) toWorkspaceAPIKey() *tenant.WorkspaceAPIKey {
	return &tenant.WorkspaceAPIKey{
		ID:          r.ID,
		WorkspaceID: r.WorkspaceID,
		Name:        r.Name,
		Scope:       r.Scope,
		KeyPrefix:   r.KeyPrefix,
		KeyHash:     r.KeyHash,
		CreatedAt:   r.CreatedAt,
		LastUsedAt:  r.LastUsedAt,
		RevokedAt:   r.RevokedAt,
	}
}

// WorkspaceAPIKeyRepo is the Postgres implementation of tenant.WorkspaceAPIKeyRepository.
type WorkspaceAPIKeyRepo struct{ db *sqlx.DB }

func NewWorkspaceAPIKeyRepo(db *sqlx.DB) *WorkspaceAPIKeyRepo {
	return &WorkspaceAPIKeyRepo{db: db}
}

func (r *WorkspaceAPIKeyRepo) Save(ctx context.Context, key *tenant.WorkspaceAPIKey) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO workspace_api_keys (id, workspace_id, name, scope, key_prefix, key_hash, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)`,
		key.ID, key.WorkspaceID, key.Name, key.Scope,
		key.KeyPrefix, key.KeyHash, key.CreatedAt)
	if err != nil {
		return fmt.Errorf("WorkspaceAPIKeyRepo.Save: %w", err)
	}
	return nil
}

func (r *WorkspaceAPIKeyRepo) GetByPrefix(ctx context.Context, keyPrefix string) (*tenant.WorkspaceAPIKey, error) {
	var row workspaceAPIKeyRow
	err := r.db.GetContext(ctx, &row, `
		SELECT id, workspace_id, name, scope, key_prefix, key_hash, created_at, last_used_at, revoked_at
		FROM workspace_api_keys
		WHERE key_prefix=$1 AND revoked_at IS NULL`, keyPrefix)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("WorkspaceAPIKeyRepo.GetByPrefix: %w", tenant.ErrWorkspaceAPIKeyNotFound)
	}
	if err != nil {
		return nil, fmt.Errorf("WorkspaceAPIKeyRepo.GetByPrefix: %w", err)
	}
	return row.toWorkspaceAPIKey(), nil
}

func (r *WorkspaceAPIKeyRepo) ListByWorkspace(ctx context.Context, workspaceID uuid.UUID, limit, offset int) ([]*tenant.WorkspaceAPIKey, int64, error) {
	var total int64
	if err := r.db.GetContext(ctx, &total, `
		SELECT COUNT(*) FROM workspace_api_keys
		WHERE workspace_id=$1 AND revoked_at IS NULL`, workspaceID); err != nil {
		return nil, 0, fmt.Errorf("WorkspaceAPIKeyRepo.ListByWorkspace.count: %w", err)
	}

	var rows []workspaceAPIKeyRow
	err := r.db.SelectContext(ctx, &rows, `
		SELECT id, workspace_id, name, scope, key_prefix, key_hash, created_at, last_used_at, revoked_at
		FROM workspace_api_keys
		WHERE workspace_id=$1 AND revoked_at IS NULL
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3`, workspaceID, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("WorkspaceAPIKeyRepo.ListByWorkspace: %w", err)
	}
	keys := make([]*tenant.WorkspaceAPIKey, len(rows))
	for i, row := range rows {
		keys[i] = row.toWorkspaceAPIKey()
	}
	return keys, total, nil
}

func (r *WorkspaceAPIKeyRepo) Revoke(ctx context.Context, id, workspaceID uuid.UUID) error {
	res, err := r.db.ExecContext(ctx, `
		UPDATE workspace_api_keys
		SET revoked_at=NOW()
		WHERE id=$1 AND workspace_id=$2 AND revoked_at IS NULL`,
		id, workspaceID)
	if err != nil {
		return fmt.Errorf("WorkspaceAPIKeyRepo.Revoke: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("WorkspaceAPIKeyRepo.Revoke: %w", tenant.ErrWorkspaceAPIKeyNotFound)
	}
	return nil
}
