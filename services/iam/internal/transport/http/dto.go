package http

import "time"

// Auth DTOs
type RegisterRequest struct {
	TenantID  string `json:"tenant_id"  validate:"required"`
	Email     string `json:"email"      validate:"required"`
	Password  string `json:"password"   validate:"required"`
	FirstName string `json:"first_name" validate:"required"`
	LastName  string `json:"last_name"  validate:"required"`
}

type LoginRequest struct {
	Email    string `json:"email"    validate:"required"`
	Password string `json:"password" validate:"required"`
}

type RefreshRequest struct {
	RefreshToken string `json:"refresh_token" validate:"required"`
}

type LogoutRequest struct {
	RefreshToken string `json:"refresh_token"`
}

type ForgotPasswordRequest struct {
	Email string `json:"email" validate:"required"`
}

type ResetPasswordRequest struct {
	Token    string `json:"token"    validate:"required"`
	Password string `json:"password" validate:"required"`
}

type VerifyEmailRequest struct {
	Token string `json:"token" validate:"required"`
}

type UpdateMeRequest struct {
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
}

type ChangePasswordRequest struct {
	CurrentPassword string `json:"current_password" validate:"required"`
	NewPassword     string `json:"new_password"     validate:"required"`
}

type TokenPairResponse struct {
	AccessToken  string    `json:"access_token"`
	RefreshToken string    `json:"refresh_token"`
	TokenType    string    `json:"token_type"`
	ExpiresAt    time.Time `json:"expires_at"`
}

type UserResponse struct {
	ID            string    `json:"id"`
	TenantID      string    `json:"tenant_id"`
	Email         string    `json:"email"`
	FirstName     string    `json:"first_name"`
	LastName      string    `json:"last_name"`
	Roles         []string  `json:"roles"`
	Status        string    `json:"status"`
	EmailVerified bool      `json:"email_verified"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

// Tenant DTOs
type CreateOrgRequest struct {
	Name        string `json:"name"          validate:"required"`
	Slug        string `json:"slug"          validate:"required"`
	OwnerUserID string `json:"owner_user_id" validate:"required"`
}

type UpdateOrgRequest struct {
	Name    string `json:"name"`
	LogoURL string `json:"logo_url"`
	Website string `json:"website"`
}

type CreateWorkspaceRequest struct {
	OrgID       string `json:"org_id"      validate:"required"`
	Name        string `json:"name"        validate:"required"`
	Slug        string `json:"slug"        validate:"required"`
	Description string `json:"description"`
}

type OrgResponse struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Slug        string    `json:"slug"`
	Plan        string    `json:"plan"`
	OwnerUserID string    `json:"owner_user_id"`
	LogoURL     string    `json:"logo_url"`
	Website     string    `json:"website"`
	CreatedAt   time.Time `json:"created_at"`
}

type UpdateWorkspaceRequest struct {
	Name        string `json:"name"`
	Slug        string `json:"slug"`
	Description string `json:"description"`
}

type WorkspaceResponse struct {
	ID          string    `json:"id"`
	OrgID       string    `json:"org_id"`
	Name        string    `json:"name"`
	Slug        string    `json:"slug"`
	Description string    `json:"description"`
	MemberCount int       `json:"member_count"`
	CreatedAt   time.Time `json:"created_at"`
}

// Workspace member DTOs
type AddMemberRequest struct {
	UserID string `json:"user_id" validate:"required"`
	Role   string `json:"role"    validate:"required"`
}

type UpdateMemberRequest struct {
	Role string `json:"role" validate:"required"`
}

type WorkspaceMemberResponse struct {
	ID          string    `json:"id"`
	WorkspaceID string    `json:"workspace_id"`
	UserID      string    `json:"user_id"`
	Name        string    `json:"name"`
	Email       string    `json:"email"`
	Role        string    `json:"role"`
	JoinedAt    time.Time `json:"joined_at"`
}

// API key DTOs
type CreateAPIKeyRequest struct {
	Name   string   `json:"name"   validate:"required"`
	Scopes []string `json:"scopes"`
}

type APIKeyResponse struct {
	ID        string     `json:"id"`
	TenantID  string     `json:"tenant_id"`
	UserID    string     `json:"user_id"`
	Name      string     `json:"name"`
	KeyPrefix string     `json:"key_prefix"`
	Scopes    []string   `json:"scopes"`
	CreatedAt time.Time  `json:"created_at"`
	LastUsed  *time.Time `json:"last_used,omitempty"`
}

type CreateAPIKeyResponse struct {
	APIKeyResponse
	Key string `json:"key"`
}

// Workspace API key DTOs

type CreateWorkspaceAPIKeyRequest struct {
	Name  string `json:"name"  validate:"required"`
	Scope string `json:"scope" validate:"required"`
}

type WorkspaceAPIKeyResponse struct {
	ID          string     `json:"id"`
	WorkspaceID string     `json:"workspace_id"`
	Name        string     `json:"name"`
	Scope       string     `json:"scope"`
	KeyPrefix   string     `json:"key_prefix"`
	CreatedAt   time.Time  `json:"created_at"`
	LastUsedAt  *time.Time `json:"last_used_at,omitempty"`
}

type CreateWorkspaceAPIKeyResponse struct {
	WorkspaceAPIKeyResponse
	Key string `json:"key"`
}

type SignupRequest struct {
	Email    string `json:"email"    validate:"required"`
	Password string `json:"password" validate:"required"`
}

type SignupResponse struct {
	AccessToken  string        `json:"access_token"`
	RefreshToken string        `json:"refresh_token"`
	TokenType    string        `json:"token_type"`
	ExpiresAt    time.Time     `json:"expires_at"`
	User         *UserResponse `json:"user"`
}
