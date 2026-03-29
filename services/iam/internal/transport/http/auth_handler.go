package http

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/greenlab/iam/internal/application"
	"github.com/greenlab/iam/internal/domain/auth"
	"github.com/greenlab/shared/pkg/apierr"
	sharedMiddleware "github.com/greenlab/shared/pkg/middleware"
	"github.com/greenlab/shared/pkg/response"
	"github.com/greenlab/shared/pkg/validator"
)

type AuthHandler struct {
	svc            *application.AuthService
	accessTokenTTL int
}

func NewAuthHandler(svc *application.AuthService) *AuthHandler {
	return &AuthHandler{
		svc:            svc,
		accessTokenTTL: int(svc.AccessTokenTTL().Seconds()),
	}
}

// Health godoc
// @Summary      Health check
// @Tags         health
// @Produce      json
// @Success      200  {object}  map[string]interface{}
// @Router       /health [get]
func (h *AuthHandler) Health(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

// Register godoc
// @Summary      Register a new user
// @Tags         auth
// @Accept       json
// @Produce      json
// @Param        request  body      RegisterRequest  true  "Registration request"
// @Success      201      {object}  UserResponse
// @Failure      400      {object}  map[string]interface{}
// @Failure      409      {object}  map[string]interface{}
// @Router       /api/v1/auth/register [post]
func (h *AuthHandler) Register(c *gin.Context) {
	var req RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, apierr.BadRequest(err.Error()))
		return
	}
	if err := validator.Validate(&req); err != nil {
		response.ValidationError(c, err)
		return
	}
	user, err := h.svc.Register(c.Request.Context(), application.RegisterInput{
		TenantID: req.TenantID, Email: req.Email, Password: req.Password,
		FirstName: req.FirstName, LastName: req.LastName,
	})
	if err != nil {
		response.Error(c, mapAuthError(err))
		return
	}
	response.Created(c, toUserResponse(user))
}

// Login godoc
// @Summary      Authenticate a user and obtain tokens
// @Tags         auth
// @Accept       json
// @Produce      json
// @Param        request  body      LoginRequest  true  "Login credentials"
// @Success      200      {object}  TokenPairResponse
// @Failure      400      {object}  map[string]interface{}
// @Failure      401      {object}  map[string]interface{}
// @Failure      403      {object}  map[string]interface{}
// @Router       /api/v1/auth/login [post]
func (h *AuthHandler) Login(c *gin.Context) {
	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, apierr.BadRequest(err.Error()))
		return
	}
	if err := validator.Validate(&req); err != nil {
		response.ValidationError(c, err)
		return
	}
	pair, err := h.svc.Login(c.Request.Context(), application.LoginInput{
		Email: req.Email, Password: req.Password,
		UserAgent: c.GetHeader("User-Agent"), IPAddress: c.ClientIP(),
	})
	if err != nil {
		response.Error(c, mapAuthError(err))
		return
	}
	response.OK(c, toTokenPairResponse(pair))
}

// Refresh godoc
// @Summary      Refresh an access token using a refresh token
// @Tags         auth
// @Accept       json
// @Produce      json
// @Param        request  body      RefreshRequest  true  "Refresh token"
// @Success      200      {object}  TokenPairResponse
// @Failure      400      {object}  map[string]interface{}
// @Failure      401      {object}  map[string]interface{}
// @Router       /api/v1/auth/refresh [post]
func (h *AuthHandler) Refresh(c *gin.Context) {
	var req RefreshRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, apierr.BadRequest(err.Error()))
		return
	}
	if err := validator.Validate(&req); err != nil {
		response.ValidationError(c, err)
		return
	}
	pair, err := h.svc.Refresh(c.Request.Context(), application.RefreshInput{
		RefreshToken: req.RefreshToken,
		UserAgent:    c.GetHeader("User-Agent"),
		IPAddress:    c.ClientIP(),
	})
	if err != nil {
		response.Error(c, apierr.Unauthorized(err.Error()))
		return
	}
	response.OK(c, toTokenPairResponse(pair))
}

// Logout godoc
// @Summary      Invalidate the current session
// @Tags         auth
// @Accept       json
// @Produce      json
// @Param        request  body      LogoutRequest  false  "Refresh token to revoke"
// @Success      204      "No Content"
// @Failure      500      {object}  map[string]interface{}
// @Security     BearerAuth
// @Router       /api/v1/auth/logout [post]
func (h *AuthHandler) Logout(c *gin.Context) {
	var req LogoutRequest
	_ = c.ShouldBindJSON(&req)
	jti, _ := c.Get("jti")
	jtiStr, _ := jti.(string)
	if err := h.svc.Logout(c.Request.Context(), req.RefreshToken, jtiStr, h.accessTokenTTL); err != nil {
		response.Error(c, apierr.Internal(err))
		return
	}
	response.NoContent(c)
}

// ForgotPassword godoc
// @Summary      Request a password reset email
// @Tags         auth
// @Accept       json
// @Produce      json
// @Param        request  body      ForgotPasswordRequest  true  "Email address"
// @Success      200      {object}  map[string]interface{}
// @Failure      400      {object}  map[string]interface{}
// @Router       /api/v1/auth/forgot-password [post]
func (h *AuthHandler) ForgotPassword(c *gin.Context) {
	var req ForgotPasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, apierr.BadRequest(err.Error()))
		return
	}
	if err := validator.Validate(&req); err != nil {
		response.ValidationError(c, err)
		return
	}
	_ = h.svc.ForgotPassword(c.Request.Context(), req.Email)
	response.OK(c, gin.H{"message": "If an account exists with that email, a reset link has been sent."})
}

// ResetPassword godoc
// @Summary      Reset password using a reset token
// @Tags         auth
// @Accept       json
// @Produce      json
// @Param        request  body      ResetPasswordRequest  true  "Reset token and new password"
// @Success      200      {object}  map[string]interface{}
// @Failure      400      {object}  map[string]interface{}
// @Router       /api/v1/auth/reset-password [post]
func (h *AuthHandler) ResetPassword(c *gin.Context) {
	var req ResetPasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, apierr.BadRequest(err.Error()))
		return
	}
	if err := validator.Validate(&req); err != nil {
		response.ValidationError(c, err)
		return
	}
	if err := h.svc.ResetPassword(c.Request.Context(), req.Token, req.Password); err != nil {
		response.Error(c, mapAuthError(err))
		return
	}
	response.OK(c, gin.H{"message": "Password has been reset successfully."})
}

// VerifyEmail godoc
// @Summary      Verify a user's email address
// @Tags         auth
// @Accept       json
// @Produce      json
// @Param        request  body      VerifyEmailRequest  true  "Verification token"
// @Success      200      {object}  map[string]interface{}
// @Failure      400      {object}  map[string]interface{}
// @Router       /api/v1/auth/verify-email [post]
func (h *AuthHandler) VerifyEmail(c *gin.Context) {
	var req VerifyEmailRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, apierr.BadRequest(err.Error()))
		return
	}
	if err := validator.Validate(&req); err != nil {
		response.ValidationError(c, err)
		return
	}
	if err := h.svc.VerifyEmail(c.Request.Context(), req.Token); err != nil {
		response.Error(c, mapAuthError(err))
		return
	}
	response.OK(c, gin.H{"message": "Email verified successfully."})
}

// GetMe godoc
// @Summary      Get the authenticated user's profile
// @Tags         auth
// @Produce      json
// @Success      200  {object}  UserResponse
// @Failure      401  {object}  map[string]interface{}
// @Failure      404  {object}  map[string]interface{}
// @Security     BearerAuth
// @Router       /api/v1/auth/me [get]
func (h *AuthHandler) GetMe(c *gin.Context) {
	userID, ok := sharedMiddleware.GetUserID(c)
	if !ok {
		response.Error(c, apierr.Unauthorized("not authenticated"))
		return
	}
	user, err := h.svc.GetMe(c.Request.Context(), userID)
	if err != nil {
		response.Error(c, mapAuthError(err))
		return
	}
	response.OK(c, toUserResponse(user))
}

// UpdateMe godoc
// @Summary      Update the authenticated user's profile
// @Tags         auth
// @Accept       json
// @Produce      json
// @Param        request  body      UpdateMeRequest  true  "Profile update"
// @Success      200      {object}  UserResponse
// @Failure      400      {object}  map[string]interface{}
// @Failure      401      {object}  map[string]interface{}
// @Security     BearerAuth
// @Router       /api/v1/auth/me [put]
func (h *AuthHandler) UpdateMe(c *gin.Context) {
	userID, ok := sharedMiddleware.GetUserID(c)
	if !ok {
		response.Error(c, apierr.Unauthorized("not authenticated"))
		return
	}
	var req UpdateMeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, apierr.BadRequest(err.Error()))
		return
	}
	if err := validator.Validate(&req); err != nil {
		response.ValidationError(c, err)
		return
	}
	user, err := h.svc.UpdateMe(c.Request.Context(), userID, application.UpdateMeInput{
		FirstName: req.FirstName, LastName: req.LastName,
	})
	if err != nil {
		response.Error(c, mapAuthError(err))
		return
	}
	response.OK(c, toUserResponse(user))
}

// ChangePassword godoc
// @Summary      Change the authenticated user's password
// @Tags         auth
// @Accept       json
// @Produce      json
// @Param        request  body      ChangePasswordRequest  true  "Current and new password"
// @Success      200      {object}  map[string]interface{}
// @Failure      400      {object}  map[string]interface{}
// @Failure      401      {object}  map[string]interface{}
// @Security     BearerAuth
// @Router       /api/v1/auth/me/password [put]
func (h *AuthHandler) ChangePassword(c *gin.Context) {
	userID, ok := sharedMiddleware.GetUserID(c)
	if !ok {
		response.Error(c, apierr.Unauthorized("not authenticated"))
		return
	}
	var req ChangePasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, apierr.BadRequest(err.Error()))
		return
	}
	if err := validator.Validate(&req); err != nil {
		response.ValidationError(c, err)
		return
	}
	if err := h.svc.ChangePassword(c.Request.Context(), userID, req.CurrentPassword, req.NewPassword); err != nil {
		response.Error(c, mapAuthError(err))
		return
	}
	response.OK(c, gin.H{"message": "Password changed successfully."})
}

// Signup godoc
// @Summary      Self-register a new account with email and password
// @Tags         auth
// @Accept       json
// @Produce      json
// @Param        request  body      SignupRequest   true  "Signup credentials"
// @Success      201      {object}  SignupResponse
// @Failure      400      {object}  map[string]interface{}
// @Failure      409      {object}  map[string]interface{}
// @Router       /api/v1/auth/signup [post]
func (h *AuthHandler) Signup(c *gin.Context) {
	var req SignupRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, apierr.BadRequest(err.Error()))
		return
	}
	if err := validator.Validate(&req); err != nil {
		response.ValidationError(c, err)
		return
	}
	result, err := h.svc.SelfSignup(c.Request.Context(), application.SelfSignupInput{
		Email:     req.Email,
		Password:  req.Password,
		UserAgent: c.GetHeader("User-Agent"),
		IPAddress: c.ClientIP(),
	})
	if err != nil {
		response.Error(c, mapAuthError(err))
		return
	}
	response.Created(c, &SignupResponse{
		AccessToken:  result.Pair.AccessToken,
		RefreshToken: result.Pair.RefreshToken,
		TokenType:    result.Pair.TokenType,
		ExpiresAt:    result.Pair.ExpiresAt,
		User:         toUserResponse(result.User),
	})
}

func toUserResponse(u *auth.User) *UserResponse {
	roles := make([]string, len(u.Roles))
	for i, r := range u.Roles {
		roles[i] = string(r)
	}
	return &UserResponse{
		ID: u.ID.String(), TenantID: u.TenantID.String(), Email: u.Email,
		FirstName: u.FirstName, LastName: u.LastName, Roles: roles,
		Status: string(u.Status), EmailVerified: u.EmailVerified,
		CreatedAt: u.CreatedAt, UpdatedAt: u.UpdatedAt,
	}
}

func toTokenPairResponse(p *auth.TokenPair) *TokenPairResponse {
	return &TokenPairResponse{
		AccessToken: p.AccessToken, RefreshToken: p.RefreshToken,
		TokenType: p.TokenType, ExpiresAt: p.ExpiresAt,
	}
}

func mapAuthError(err error) error {
	switch {
	case errors.Is(err, auth.ErrUserNotFound):
		return apierr.NotFound("user")
	case errors.Is(err, auth.ErrEmailAlreadyRegistered):
		return apierr.Conflict(err.Error())
	case errors.Is(err, auth.ErrInvalidCredentials):
		return apierr.Unauthorized("invalid credentials")
	case errors.Is(err, auth.ErrAccountIsNotActive):
		return apierr.Forbidden("account is not active")
	case errors.Is(err, auth.ErrInvalidPassword):
		return apierr.BadRequest("current password is incorrect")
	case errors.Is(err, auth.ErrInvalidResetToken):
		return apierr.BadRequest("invalid or expired reset token")
	case errors.Is(err, auth.ErrInvalidToken),
		errors.Is(err, auth.ErrTokenNotFound),
		errors.Is(err, auth.ErrTokenExpired):
		return apierr.Unauthorized("invalid or expired token")
	case errors.Is(err, auth.ErrInvalidEmail):
		return apierr.BadRequest("invalid email address")
	default:
		return apierr.Internal(err)
	}
}
