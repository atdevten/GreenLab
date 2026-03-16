package auth

import "errors"

var (
	ErrUserNotFound       = errors.New("user not found")
	ErrInvalidCredentials = errors.New("invalid credentials")
	ErrTokenNotFound      = errors.New("token not found")
	ErrTokenExpired       = errors.New("token expired")
	ErrInvalidToken       = errors.New("invalid token")
	ErrAccountIsNotActive = errors.New("account is not active")
	ErrGetRolesFailed     = errors.New("failed to get user roles")
	ErrIssueTokenFailed   = errors.New("failed to issue token")

	ErrEmailAlreadyRegistered = errors.New("email already registered")
	ErrInvalidResetToken      = errors.New("invalid or expired reset token")
	ErrInvalidPassword        = errors.New("invalid password")
	ErrInvalidEmail           = errors.New("invalid email address")
)
