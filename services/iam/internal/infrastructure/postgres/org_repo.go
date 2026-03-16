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

type orgRow struct {
	ID          uuid.UUID      `db:"id"`
	Name        string         `db:"name"`
	Slug        string         `db:"slug"`
	Plan        tenant.OrgPlan `db:"plan"`
	OwnerUserID uuid.UUID      `db:"owner_user_id"`
	LogoURL     string         `db:"logo_url"`
	Website     string         `db:"website"`
	CreatedAt   time.Time      `db:"created_at"`
	UpdatedAt   time.Time      `db:"updated_at"`
}

func fromOrg(o *tenant.Org) orgRow {
	return orgRow{
		ID:          o.ID,
		Name:        o.Name,
		Slug:        o.Slug,
		Plan:        o.Plan,
		OwnerUserID: o.OwnerUserID,
		LogoURL:     o.LogoURL,
		Website:     o.Website,
		CreatedAt:   o.CreatedAt,
		UpdatedAt:   o.UpdatedAt,
	}
}

func (r orgRow) toOrg() *tenant.Org {
	return &tenant.Org{
		ID:          r.ID,
		Name:        r.Name,
		Slug:        r.Slug,
		Plan:        r.Plan,
		OwnerUserID: r.OwnerUserID,
		LogoURL:     r.LogoURL,
		Website:     r.Website,
		CreatedAt:   r.CreatedAt,
		UpdatedAt:   r.UpdatedAt,
	}
}

type OrgRepo struct{ db *sqlx.DB }

func NewOrgRepo(db *sqlx.DB) *OrgRepo { return &OrgRepo{db: db} }

func (r *OrgRepo) Create(ctx context.Context, org *tenant.Org) error {
	row := fromOrg(org)
	_, err := r.db.NamedExecContext(ctx, `
		INSERT INTO orgs (id, name, slug, plan, owner_user_id, logo_url, website)
		VALUES (:id, :name, :slug, :plan, :owner_user_id, :logo_url, :website)`, row)
	if err != nil {
		if strings.Contains(err.Error(), "unique") || strings.Contains(err.Error(), "duplicate") {
			return fmt.Errorf("OrgRepo.Create: %w: %w", tenant.ErrSlugAlreadyTaken, err)
		}
		return fmt.Errorf("OrgRepo.Create: %w", err)
	}
	return nil
}

func (r *OrgRepo) GetByID(ctx context.Context, id uuid.UUID) (*tenant.Org, error) {
	var row orgRow
	err := r.db.GetContext(ctx, &row,
		`SELECT id, name, slug, plan, owner_user_id, logo_url, website, created_at, updated_at
		 FROM orgs WHERE id = $1`, id)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("OrgRepo.GetByID: %w", tenant.ErrOrgNotFound)
	}
	if err != nil {
		return nil, fmt.Errorf("OrgRepo.GetByID: %w", err)
	}
	return row.toOrg(), nil
}

func (r *OrgRepo) GetBySlug(ctx context.Context, slug string) (*tenant.Org, error) {
	var row orgRow
	err := r.db.GetContext(ctx, &row,
		`SELECT id, name, slug, plan, owner_user_id, logo_url, website, created_at, updated_at
		 FROM orgs WHERE slug = $1`, slug)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("OrgRepo.GetBySlug: %w", tenant.ErrOrgNotFound)
	}
	if err != nil {
		return nil, fmt.Errorf("OrgRepo.GetBySlug: %w", err)
	}
	return row.toOrg(), nil
}

func (r *OrgRepo) List(ctx context.Context, limit, offset int) ([]*tenant.Org, int64, error) {
	var rows []orgRow
	err := r.db.SelectContext(ctx, &rows,
		`SELECT id, name, slug, plan, owner_user_id, logo_url, website, created_at, updated_at
		 FROM orgs ORDER BY created_at DESC LIMIT $1 OFFSET $2`, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	orgs := make([]*tenant.Org, len(rows))
	for i, row := range rows {
		orgs[i] = row.toOrg()
	}
	var total int64
	if err := r.db.GetContext(ctx, &total, `SELECT COUNT(*) FROM orgs`); err != nil {
		return nil, 0, fmt.Errorf("OrgRepo.List count: %w", err)
	}
	return orgs, total, nil
}

func (r *OrgRepo) Update(ctx context.Context, org *tenant.Org) error {
	_, err := r.db.ExecContext(ctx, `
		UPDATE orgs SET name=$1, logo_url=$2, website=$3, updated_at=NOW() WHERE id=$4`,
		org.Name, org.LogoURL, org.Website, org.ID)
	return err
}

func (r *OrgRepo) Delete(ctx context.Context, id uuid.UUID) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM orgs WHERE id=$1`, id)
	return err
}
