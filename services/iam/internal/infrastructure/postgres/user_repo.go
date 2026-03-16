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
	"github.com/greenlab/iam/internal/domain/auth"
)

type userRow struct {
	ID               uuid.UUID       `db:"id"`
	TenantID         uuid.UUID       `db:"tenant_id"`
	Email            string          `db:"email"`
	PasswordHash     string          `db:"password_hash"`
	FirstName        string          `db:"first_name"`
	LastName         string          `db:"last_name"`
	Status           auth.UserStatus `db:"status"`
	EmailVerified    bool            `db:"email_verified"`
	VerifyToken      string          `db:"verify_token"`
	ResetToken       string          `db:"reset_token"`
	ResetTokenExpiry *time.Time      `db:"reset_token_expiry"`
	CreatedAt        time.Time       `db:"created_at"`
	UpdatedAt        time.Time       `db:"updated_at"`
}

func fromUser(u *auth.User) userRow {
	return userRow{
		ID:               u.ID,
		TenantID:         u.TenantID,
		Email:            u.Email,
		PasswordHash:     u.PasswordHash,
		FirstName:        u.FirstName,
		LastName:         u.LastName,
		Status:           u.Status,
		EmailVerified:    u.EmailVerified,
		VerifyToken:      u.VerifyToken,
		ResetToken:       u.ResetToken,
		ResetTokenExpiry: u.ResetTokenExpiry,
		CreatedAt:        u.CreatedAt,
		UpdatedAt:        u.UpdatedAt,
	}
}

func (r userRow) toUser() *auth.User {
	return &auth.User{
		ID:               r.ID,
		TenantID:         r.TenantID,
		Email:            r.Email,
		PasswordHash:     r.PasswordHash,
		FirstName:        r.FirstName,
		LastName:         r.LastName,
		Status:           r.Status,
		EmailVerified:    r.EmailVerified,
		VerifyToken:      r.VerifyToken,
		ResetToken:       r.ResetToken,
		ResetTokenExpiry: r.ResetTokenExpiry,
		CreatedAt:        r.CreatedAt,
		UpdatedAt:        r.UpdatedAt,
	}
}

type UserRepo struct{ db *sqlx.DB }

func NewUserRepo(db *sqlx.DB) *UserRepo { return &UserRepo{db: db} }

func (r *UserRepo) Create(ctx context.Context, u *auth.User) error {
	row := fromUser(u)
	_, err := r.db.NamedExecContext(ctx, `
		INSERT INTO users (id, tenant_id, email, password_hash, first_name, last_name, status, email_verified, verify_token)
		VALUES (:id, :tenant_id, :email, :password_hash, :first_name, :last_name, :status, :email_verified, :verify_token)`, row)
	if err != nil {
		if strings.Contains(err.Error(), "unique") || strings.Contains(err.Error(), "duplicate") {
			return fmt.Errorf("UserRepo.Create: %w: %w", auth.ErrEmailAlreadyRegistered, err)
		}
		return fmt.Errorf("UserRepo.Create: %w", err)
	}
	return nil
}

func (r *UserRepo) GetByID(ctx context.Context, id uuid.UUID) (*auth.User, error) {
	var row userRow
	err := r.db.GetContext(ctx, &row,
		`SELECT id, tenant_id, email, password_hash, first_name, last_name,
		        status, email_verified, verify_token, reset_token, reset_token_expiry,
		        created_at, updated_at
		 FROM users WHERE id = $1`, id)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("UserRepo.GetByID: %w", auth.ErrUserNotFound)
	}
	if err != nil {
		return nil, fmt.Errorf("UserRepo.GetByID: %w", err)
	}
	return row.toUser(), nil
}

func (r *UserRepo) GetByEmail(ctx context.Context, email string) (*auth.User, error) {
	var row userRow
	err := r.db.GetContext(ctx, &row,
		`SELECT id, tenant_id, email, password_hash, first_name, last_name,
		        status, email_verified, verify_token, reset_token, reset_token_expiry,
		        created_at, updated_at
		 FROM users WHERE email = $1`, email)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("UserRepo.GetByEmail: %w", auth.ErrUserNotFound)
	}
	if err != nil {
		return nil, fmt.Errorf("UserRepo.GetByEmail: %w", err)
	}
	return row.toUser(), nil
}

func (r *UserRepo) GetByVerifyToken(ctx context.Context, token string) (*auth.User, error) {
	var row userRow
	err := r.db.GetContext(ctx, &row,
		`SELECT id, tenant_id, email, password_hash, first_name, last_name,
		        status, email_verified, verify_token, reset_token, reset_token_expiry,
		        created_at, updated_at
		 FROM users WHERE verify_token = $1`, token)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("UserRepo.GetByVerifyToken: %w", auth.ErrUserNotFound)
	}
	if err != nil {
		return nil, fmt.Errorf("UserRepo.GetByVerifyToken: %w", err)
	}
	return row.toUser(), nil
}

func (r *UserRepo) GetByResetToken(ctx context.Context, token string) (*auth.User, error) {
	var row userRow
	err := r.db.GetContext(ctx, &row,
		`SELECT id, tenant_id, email, password_hash, first_name, last_name,
		        status, email_verified, verify_token, reset_token, reset_token_expiry,
		        created_at, updated_at
		 FROM users WHERE reset_token = $1 AND reset_token_expiry > NOW()`, token)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("UserRepo.GetByResetToken: %w", auth.ErrInvalidResetToken)
	}
	if err != nil {
		return nil, fmt.Errorf("UserRepo.GetByResetToken: %w", err)
	}
	return row.toUser(), nil
}

func (r *UserRepo) Update(ctx context.Context, u *auth.User) error {
	_, err := r.db.ExecContext(ctx, `
		UPDATE users SET
			first_name = $1, last_name = $2, password_hash = $3,
			status = $4, email_verified = $5, verify_token = $6,
			reset_token = $7, reset_token_expiry = $8, updated_at = NOW()
		WHERE id = $9`,
		u.FirstName, u.LastName, u.PasswordHash,
		u.Status, u.EmailVerified, u.VerifyToken,
		u.ResetToken, u.ResetTokenExpiry, u.ID,
	)
	return err
}

func (r *UserRepo) Delete(ctx context.Context, id uuid.UUID) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM users WHERE id = $1`, id)
	return err
}

func (r *UserRepo) GetRoles(ctx context.Context, userID uuid.UUID) ([]auth.Role, error) {
	var roles []string
	err := r.db.SelectContext(ctx, &roles, `SELECT role FROM user_roles WHERE user_id = $1`, userID)
	if err != nil {
		return nil, err
	}
	result := make([]auth.Role, len(roles))
	for i, ro := range roles {
		result[i] = auth.Role(ro)
	}
	return result, nil
}

func (r *UserRepo) SetRoles(ctx context.Context, userID uuid.UUID, roles []auth.Role) (err error) {
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()
	if _, err = tx.ExecContext(ctx, `DELETE FROM user_roles WHERE user_id = $1`, userID); err != nil {
		return err
	}
	for _, role := range roles {
		if _, err = tx.ExecContext(ctx, `INSERT INTO user_roles (user_id, role) VALUES ($1, $2)`, userID, string(role)); err != nil {
			return err
		}
	}
	return tx.Commit()
}
