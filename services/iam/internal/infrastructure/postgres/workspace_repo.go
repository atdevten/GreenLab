package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/greenlab/iam/internal/domain/tenant"
)

type workspaceRow struct {
	ID          uuid.UUID `db:"id"`
	OrgID       uuid.UUID `db:"org_id"`
	Name        string    `db:"name"`
	Slug        string    `db:"slug"`
	Description string    `db:"description"`
	CreatedAt   time.Time `db:"created_at"`
	UpdatedAt   time.Time `db:"updated_at"`
}

func fromWorkspace(ws *tenant.Workspace) workspaceRow {
	return workspaceRow{
		ID:          ws.ID,
		OrgID:       ws.OrgID,
		Name:        ws.Name,
		Slug:        ws.Slug,
		Description: ws.Description,
		CreatedAt:   ws.CreatedAt,
		UpdatedAt:   ws.UpdatedAt,
	}
}

func (r workspaceRow) toWorkspace() *tenant.Workspace {
	return &tenant.Workspace{
		ID:          r.ID,
		OrgID:       r.OrgID,
		Name:        r.Name,
		Slug:        r.Slug,
		Description: r.Description,
		CreatedAt:   r.CreatedAt,
		UpdatedAt:   r.UpdatedAt,
	}
}

type WorkspaceRepo struct{ db *sqlx.DB }

func NewWorkspaceRepo(db *sqlx.DB) *WorkspaceRepo { return &WorkspaceRepo{db: db} }

func (r *WorkspaceRepo) Create(ctx context.Context, ws *tenant.Workspace) error {
	row := fromWorkspace(ws)
	_, err := r.db.NamedExecContext(ctx, `
		INSERT INTO workspaces (id, org_id, name, slug, description)
		VALUES (:id, :org_id, :name, :slug, :description)`, row)
	if err != nil {
		if strings.Contains(err.Error(), "unique") || strings.Contains(err.Error(), "duplicate") {
			return fmt.Errorf("WorkspaceRepo.Create: %w", tenant.ErrSlugAlreadyTaken)
		}
		return fmt.Errorf("WorkspaceRepo.Create: %w", err)
	}
	return nil
}

func (r *WorkspaceRepo) GetByID(ctx context.Context, id uuid.UUID) (*tenant.Workspace, error) {
	var row workspaceRow
	err := r.db.GetContext(ctx, &row,
		`SELECT id, org_id, name, slug, description, created_at, updated_at
		 FROM workspaces WHERE id=$1`, id)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("WorkspaceRepo.GetByID: %w", tenant.ErrWorkspaceNotFound)
	}
	if err != nil {
		return nil, fmt.Errorf("WorkspaceRepo.GetByID: %w", err)
	}
	return row.toWorkspace(), nil
}

func (r *WorkspaceRepo) ListByOrg(ctx context.Context, orgID uuid.UUID) ([]*tenant.Workspace, error) {
	var rows []workspaceRow
	err := r.db.SelectContext(ctx, &rows,
		`SELECT id, org_id, name, slug, description, created_at, updated_at
		 FROM workspaces WHERE org_id=$1 ORDER BY created_at`, orgID)
	if err != nil {
		return nil, err
	}
	wss := make([]*tenant.Workspace, len(rows))
	for i, row := range rows {
		wss[i] = row.toWorkspace()
	}
	return wss, nil
}

func (r *WorkspaceRepo) Update(ctx context.Context, ws *tenant.Workspace) error {
	_, err := r.db.ExecContext(ctx, `
		UPDATE workspaces SET name=$1, slug=$2, description=$3, updated_at=NOW() WHERE id=$4`,
		ws.Name, ws.Slug, ws.Description, ws.ID)
	if err != nil {
		if strings.Contains(err.Error(), "unique") || strings.Contains(err.Error(), "duplicate") {
			return fmt.Errorf("WorkspaceRepo.Update: %w", tenant.ErrSlugAlreadyTaken)
		}
		return fmt.Errorf("WorkspaceRepo.Update: %w", err)
	}
	return nil
}

func (r *WorkspaceRepo) Delete(ctx context.Context, id uuid.UUID) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM workspaces WHERE id=$1`, id)
	return err
}

func (r *WorkspaceRepo) ListMembers(ctx context.Context, workspaceID string) ([]tenant.WorkspaceMember, error) {
	var rows []tenant.WorkspaceMember
	err := r.db.SelectContext(ctx, &rows, `
		SELECT wm.id, wm.workspace_id, wm.user_id, u.first_name || ' ' || u.last_name AS name,
		       u.email, wm.role, wm.joined_at
		FROM workspace_members wm
		JOIN users u ON u.id = wm.user_id
		WHERE wm.workspace_id = $1
		ORDER BY wm.joined_at`, workspaceID)
	if err != nil {
		return nil, fmt.Errorf("WorkspaceRepo.ListMembers: %w", err)
	}
	return rows, nil
}

func (r *WorkspaceRepo) AddMember(ctx context.Context, m tenant.WorkspaceMember) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO workspace_members (id, workspace_id, user_id, role, joined_at)
		VALUES ($1, $2, $3, $4, $5)`,
		m.ID, m.WorkspaceID, m.UserID, m.Role, m.JoinedAt)
	if err != nil {
		if strings.Contains(err.Error(), "unique") || strings.Contains(err.Error(), "duplicate") {
			return fmt.Errorf("WorkspaceRepo.AddMember: %w", tenant.ErrMemberAlreadyExists)
		}
		return fmt.Errorf("WorkspaceRepo.AddMember: %w", err)
	}
	return nil
}

func (r *WorkspaceRepo) UpdateMember(ctx context.Context, workspaceID, userID, role string) error {
	res, err := r.db.ExecContext(ctx, `
		UPDATE workspace_members SET role=$1 WHERE workspace_id=$2 AND user_id=$3`,
		role, workspaceID, userID)
	if err != nil {
		return fmt.Errorf("WorkspaceRepo.UpdateMember: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("WorkspaceRepo.UpdateMember: %w", tenant.ErrMemberNotFound)
	}
	return nil
}

func (r *WorkspaceRepo) RemoveMember(ctx context.Context, workspaceID, userID string) error {
	res, err := r.db.ExecContext(ctx, `
		DELETE FROM workspace_members WHERE workspace_id=$1 AND user_id=$2`,
		workspaceID, userID)
	if err != nil {
		return fmt.Errorf("WorkspaceRepo.RemoveMember: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("WorkspaceRepo.RemoveMember: %w", tenant.ErrMemberNotFound)
	}
	return nil
}
