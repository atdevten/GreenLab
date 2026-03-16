package auth

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net/mail"
	"time"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

type Role string

const (
	RoleAdmin    Role = "admin"
	RoleOperator Role = "operator"
	RoleViewer   Role = "viewer"
)

type UserStatus string

const (
	UserStatusPending  UserStatus = "pending"
	UserStatusActive   UserStatus = "active"
	UserStatusDisabled UserStatus = "disabled"
)

type User struct {
	ID               uuid.UUID
	TenantID         uuid.UUID
	Email            string
	PasswordHash     string
	FirstName        string
	LastName         string
	Roles            []Role
	Status           UserStatus
	EmailVerified    bool
	VerifyToken      string
	ResetToken       string
	ResetTokenExpiry *time.Time
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

// hashPasswordInput SHA256-hashes the password before bcrypt to prevent silent
// truncation of passwords longer than bcrypt's 72-byte limit.
func hashPasswordInput(password string) []byte {
	h := sha256.Sum256([]byte(password))
	encoded := hex.EncodeToString(h[:]) // 64 ASCII bytes, always under 72
	return []byte(encoded)
}

func NewUser(tenantID uuid.UUID, email, password, firstName, lastName string) (*User, error) {
	if _, err := mail.ParseAddress(email); err != nil {
		return nil, ErrInvalidEmail
	}
	if firstName == "" {
		return nil, errors.New("first name is required")
	}
	if lastName == "" {
		return nil, errors.New("last name is required")
	}
	if len(password) < 8 {
		return nil, errors.New("password must be at least 8 characters")
	}
	hash, err := bcrypt.GenerateFromPassword(hashPasswordInput(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	return &User{
		ID:            uuid.New(),
		TenantID:      tenantID,
		Email:         email,
		PasswordHash:  string(hash),
		FirstName:     firstName,
		LastName:      lastName,
		Roles:         []Role{RoleViewer},
		Status:        UserStatusPending,
		EmailVerified: false,
		VerifyToken:   uuid.New().String(),
		CreatedAt:     now,
		UpdatedAt:     now,
	}, nil
}

func (u *User) CheckPassword(password string) bool {
	return bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), hashPasswordInput(password)) == nil
}

func (u *User) SetPassword(password string) error {
	if len(password) < 8 {
		return errors.New("password must be at least 8 characters")
	}
	hash, err := bcrypt.GenerateFromPassword(hashPasswordInput(password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	u.PasswordHash = string(hash)
	u.UpdatedAt = time.Now().UTC()
	return nil
}

func (u *User) FullName() string { return u.FirstName + " " + u.LastName }

func (u *User) IsActive() bool { return u.Status == UserStatusActive }

func (u *User) Activate() {
	u.Status = UserStatusActive
	u.EmailVerified = true
	u.VerifyToken = ""
	u.UpdatedAt = time.Now().UTC()
}

func (u *User) Disable() {
	u.Status = UserStatusDisabled
	u.UpdatedAt = time.Now().UTC()
}

func (u *User) SetResetToken(token string, expiry time.Time) {
	u.ResetToken = token
	u.ResetTokenExpiry = &expiry
	u.UpdatedAt = time.Now().UTC()
}

func (u *User) ClearResetToken() {
	u.ResetToken = ""
	u.ResetTokenExpiry = nil
	u.UpdatedAt = time.Now().UTC()
}

func (u *User) IsResetTokenValid(token string) bool {
	if u.ResetToken == "" || token == "" || u.ResetToken != token || u.ResetTokenExpiry == nil {
		return false
	}
	return time.Now().UTC().Before(*u.ResetTokenExpiry)
}

func (u *User) HasRole(role Role) bool {
	for _, r := range u.Roles {
		if r == role {
			return true
		}
	}
	return false
}
