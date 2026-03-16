package application

import (
	"context"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"regexp"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/greenlab/iam/internal/domain/auth"
	tenantDomain "github.com/greenlab/iam/internal/domain/tenant"
	sharedMiddleware "github.com/greenlab/shared/pkg/middleware"
)

const (
	accessTokenTTL  = 15 * time.Minute
	refreshTokenTTL = 7 * 24 * time.Hour
)

type AuthService struct {
	userRepo   auth.UserRepository
	tokenRepo  auth.TokenRepository
	cache      auth.CacheRepository
	publisher  auth.EventPublisher
	orgRepo    tenantDomain.OrgRepository
	privateKey *rsa.PrivateKey
	publicKey  *rsa.PublicKey
	issuer     string
	logger     *slog.Logger
}

func NewAuthService(
	userRepo auth.UserRepository,
	tokenRepo auth.TokenRepository,
	cache auth.CacheRepository,
	publisher auth.EventPublisher,
	orgRepo tenantDomain.OrgRepository,
	privateKey *rsa.PrivateKey,
	publicKey *rsa.PublicKey,
	issuer string,
) *AuthService {
	return &AuthService{
		userRepo:   userRepo,
		tokenRepo:  tokenRepo,
		cache:      cache,
		publisher:  publisher,
		orgRepo:    orgRepo,
		privateKey: privateKey,
		publicKey:  publicKey,
		issuer:     issuer,
		logger:     slog.Default(),
	}
}

type RegisterInput struct {
	TenantID  string
	Email     string
	Password  string
	FirstName string
	LastName  string
}

func (s *AuthService) Register(ctx context.Context, in RegisterInput) (*auth.User, error) {
	tenantID, err := uuid.Parse(in.TenantID)
	if err != nil {
		return nil, fmt.Errorf("Register.ParseTenantID: %w", err)
	}
	existing, err := s.userRepo.GetByEmail(ctx, in.Email)
	if err == nil && existing != nil {
		return nil, fmt.Errorf("Register.CheckEmail: %w", auth.ErrEmailAlreadyRegistered)
	}
	user, err := auth.NewUser(tenantID, in.Email, in.Password, in.FirstName, in.LastName)
	if err != nil {
		return nil, fmt.Errorf("Register.NewUser: %w", err)
	}

	// Hash the verify token before storing; return raw token on the user struct for the caller
	rawVerifyToken := user.VerifyToken
	user.VerifyToken = hashToken(rawVerifyToken)
	if err := s.userRepo.Create(ctx, user); err != nil {
		return nil, fmt.Errorf("Register.Create: %w", err)
	}
	// Restore raw token so the caller can send it to the user via email
	user.VerifyToken = rawVerifyToken

	if err := s.userRepo.SetRoles(ctx, user.ID, user.Roles); err != nil {
		return nil, fmt.Errorf("Register.SetRoles: %w", err)
	}
	if err := s.publisher.PublishUserRegistered(ctx, user); err != nil {
		s.logger.Error("failed to publish user.registered event", "user_id", user.ID, "error", err)
	}
	return user, nil
}

type LoginInput struct {
	Email     string
	Password  string
	UserAgent string
	IPAddress string
}

func (s *AuthService) Login(ctx context.Context, in LoginInput) (*auth.TokenPair, error) {
	user, err := s.userRepo.GetByEmail(ctx, in.Email)
	if err != nil {
		if errors.Is(err, auth.ErrUserNotFound) {
			return nil, fmt.Errorf("Login: %w", auth.ErrInvalidCredentials)
		}
		return nil, fmt.Errorf("Login.GetByEmail: %w", err)
	}
	if !user.CheckPassword(in.Password) {
		return nil, fmt.Errorf("Login: %w", auth.ErrInvalidCredentials)
	}
	if !user.IsActive() {
		return nil, fmt.Errorf("Login.IsActive: %w", auth.ErrAccountIsNotActive)
	}
	roles, err := s.userRepo.GetRoles(ctx, user.ID)
	if err != nil {
		return nil, fmt.Errorf("Login.GetRoles: %w: %w", auth.ErrGetRolesFailed, err)
	}
	user.Roles = roles
	pair, err := s.issueTokenPair(ctx, user, uuid.New(), in.UserAgent, in.IPAddress)
	if err != nil {
		return nil, fmt.Errorf("Login.issueTokenPair: %w: %w", auth.ErrIssueTokenFailed, err)
	}
	if err := s.publisher.PublishUserLoggedIn(ctx, user, in.IPAddress); err != nil {
		s.logger.Error("failed to publish user.logged_in event", "user_id", user.ID, "error", err)
	}
	return pair, nil
}

type RefreshInput struct {
	RefreshToken string
	UserAgent    string
	IPAddress    string
}

func (s *AuthService) Refresh(ctx context.Context, in RefreshInput) (*auth.TokenPair, error) {
	hash := hashToken(in.RefreshToken)
	stored, err := s.tokenRepo.GetAndRevokeRefreshToken(ctx, hash)
	if err != nil {
		return nil, fmt.Errorf("Refresh.GetAndRevokeRefreshToken: %w", auth.ErrInvalidToken)
	}
	user, err := s.userRepo.GetByID(ctx, stored.UserID)
	if err != nil {
		return nil, fmt.Errorf("Refresh.GetByID: %w", err)
	}
	if !user.IsActive() {
		return nil, fmt.Errorf("Refresh.IsActive: %w", auth.ErrAccountIsNotActive)
	}
	roles, err := s.userRepo.GetRoles(ctx, user.ID)
	if err != nil {
		return nil, fmt.Errorf("Refresh.GetRoles: %w: %w", auth.ErrGetRolesFailed, err)
	}
	user.Roles = roles
	pair, err := s.issueTokenPair(ctx, user, stored.Family, in.UserAgent, in.IPAddress)
	if err != nil {
		return nil, fmt.Errorf("Refresh.issueTokenPair: %w: %w", auth.ErrIssueTokenFailed, err)
	}
	return pair, nil
}

func (s *AuthService) Logout(ctx context.Context, refreshToken, jti string, jtiTTL int) error {
	if refreshToken != "" {
		hash := hashToken(refreshToken)
		if err := s.tokenRepo.RevokeRefreshToken(ctx, hash); err != nil {
			s.logger.Error("failed to revoke refresh token on logout", "error", err)
		}
	}
	if jti != "" {
		if err := s.cache.BlacklistToken(ctx, jti, jtiTTL); err != nil {
			s.logger.Error("failed to blacklist jti on logout", "jti", jti, "error", err)
		}
	}
	return nil
}

func (s *AuthService) ForgotPassword(ctx context.Context, email string) error {
	user, err := s.userRepo.GetByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, auth.ErrUserNotFound) {
			// Return nil to prevent email enumeration
			return nil
		}
		return fmt.Errorf("ForgotPassword.GetByEmail: %w", err)
	}
	rawToken := uuid.New().String()
	expiry := time.Now().UTC().Add(1 * time.Hour)
	// Store the hash; the raw token is what gets sent to the user
	user.SetResetToken(hashToken(rawToken), expiry)
	if err := s.userRepo.Update(ctx, user); err != nil {
		return fmt.Errorf("ForgotPassword.Update: %w", err)
	}
	// TODO: send rawToken to user via email
	return nil
}

func (s *AuthService) ResetPassword(ctx context.Context, token, newPassword string) error {
	tokenHash := hashToken(token)
	user, err := s.userRepo.GetByResetToken(ctx, tokenHash)
	if err != nil {
		return fmt.Errorf("ResetPassword.GetByResetToken: %w", auth.ErrInvalidResetToken)
	}
	if !user.IsResetTokenValid(tokenHash) {
		return fmt.Errorf("ResetPassword.IsResetTokenValid: %w", auth.ErrInvalidResetToken)
	}
	if err := user.SetPassword(newPassword); err != nil {
		return fmt.Errorf("ResetPassword.SetPassword: %w", err)
	}
	user.ClearResetToken()
	if err := s.userRepo.Update(ctx, user); err != nil {
		return fmt.Errorf("ResetPassword.Update: %w", err)
	}
	if err := s.tokenRepo.RevokeAllForUser(ctx, user.ID); err != nil {
		s.logger.Error("failed to revoke all tokens after password reset", "user_id", user.ID, "error", err)
	}
	if err := s.publisher.PublishPasswordChanged(ctx, user.ID); err != nil {
		s.logger.Error("failed to publish password_changed event", "user_id", user.ID, "error", err)
	}
	return nil
}

func (s *AuthService) VerifyEmail(ctx context.Context, token string) error {
	tokenHash := hashToken(token)
	user, err := s.userRepo.GetByVerifyToken(ctx, tokenHash)
	if err != nil {
		return fmt.Errorf("VerifyEmail.GetByVerifyToken: %w", auth.ErrInvalidToken)
	}
	if user.EmailVerified {
		return nil
	}
	user.Activate()
	if err := s.userRepo.Update(ctx, user); err != nil {
		return fmt.Errorf("VerifyEmail.Update: %w", err)
	}
	if err := s.publisher.PublishEmailVerified(ctx, user.ID); err != nil {
		s.logger.Error("failed to publish email_verified event", "user_id", user.ID, "error", err)
	}
	return nil
}

func (s *AuthService) GetMe(ctx context.Context, userID string) (*auth.User, error) {
	id, err := uuid.Parse(userID)
	if err != nil {
		return nil, fmt.Errorf("GetMe.ParseUserID: %w", err)
	}
	user, err := s.userRepo.GetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("GetMe.GetByID: %w", err)
	}
	roles, err := s.userRepo.GetRoles(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("GetMe.GetRoles: %w: %w", auth.ErrGetRolesFailed, err)
	}
	user.Roles = roles
	return user, nil
}

type UpdateMeInput struct {
	FirstName string
	LastName  string
}

func (s *AuthService) UpdateMe(ctx context.Context, userID string, in UpdateMeInput) (*auth.User, error) {
	id, err := uuid.Parse(userID)
	if err != nil {
		return nil, fmt.Errorf("UpdateMe.ParseUserID: %w", err)
	}
	user, err := s.userRepo.GetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("UpdateMe.GetByID: %w", err)
	}
	if in.FirstName != "" {
		user.FirstName = in.FirstName
	}
	if in.LastName != "" {
		user.LastName = in.LastName
	}
	user.UpdatedAt = time.Now().UTC()
	if err := s.userRepo.Update(ctx, user); err != nil {
		return nil, fmt.Errorf("UpdateMe.Update: %w", err)
	}
	return user, nil
}

func (s *AuthService) ChangePassword(ctx context.Context, userID, currentPassword, newPassword string) error {
	id, err := uuid.Parse(userID)
	if err != nil {
		return fmt.Errorf("ChangePassword.ParseUserID: %w", err)
	}
	user, err := s.userRepo.GetByID(ctx, id)
	if err != nil {
		return fmt.Errorf("ChangePassword.GetByID: %w", err)
	}
	if !user.CheckPassword(currentPassword) {
		return fmt.Errorf("ChangePassword.CheckPassword: %w", auth.ErrInvalidPassword)
	}
	if err := user.SetPassword(newPassword); err != nil {
		return fmt.Errorf("ChangePassword.SetPassword: %w", err)
	}
	if err := s.userRepo.Update(ctx, user); err != nil {
		return fmt.Errorf("ChangePassword.Update: %w", err)
	}
	if err := s.tokenRepo.RevokeAllForUser(ctx, user.ID); err != nil {
		s.logger.Error("failed to revoke all tokens after password change", "user_id", user.ID, "error", err)
	}
	if err := s.publisher.PublishPasswordChanged(ctx, user.ID); err != nil {
		s.logger.Error("failed to publish password_changed event", "user_id", user.ID, "error", err)
	}
	return nil
}

func (s *AuthService) AccessTokenTTL() time.Duration {
	return accessTokenTTL
}

func (s *AuthService) issueTokenPair(ctx context.Context, user *auth.User, family uuid.UUID, userAgent, ip string) (*auth.TokenPair, error) {
	jti := uuid.New().String()
	now := time.Now().UTC()
	expiresAt := now.Add(accessTokenTTL)

	roleStrings := make([]string, len(user.Roles))
	for i, r := range user.Roles {
		roleStrings[i] = string(r)
	}

	claims := sharedMiddleware.Claims{
		UserID:   user.ID.String(),
		TenantID: user.TenantID.String(),
		Email:    user.Email,
		Roles:    roleStrings,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    s.issuer,
			Subject:   user.ID.String(),
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(expiresAt),
			ID:        jti,
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	accessToken, err := token.SignedString(s.privateKey)
	if err != nil {
		return nil, fmt.Errorf("sign access token: %w", err)
	}

	rawRefresh := uuid.New().String()
	refreshHash := hashToken(rawRefresh)

	rt := auth.NewRefreshToken(user.ID, refreshHash, family, refreshTokenTTL, userAgent, ip)
	if err := s.tokenRepo.SaveRefreshToken(ctx, rt); err != nil {
		return nil, fmt.Errorf("save refresh token: %w", err)
	}

	return &auth.TokenPair{
		AccessToken:  accessToken,
		RefreshToken: rawRefresh,
		ExpiresAt:    expiresAt,
		TokenType:    "Bearer",
	}, nil
}

func hashToken(token string) string {
	h := sha256.Sum256([]byte(token))
	return hex.EncodeToString(h[:])
}

var nonAlphanumeric = regexp.MustCompile(`[^a-z0-9]+`)

type SelfSignupInput struct {
	Email     string
	Password  string
	UserAgent string
	IPAddress string
}

type SelfSignupResult struct {
	Pair *auth.TokenPair
	User *auth.User
}

func (s *AuthService) SelfSignup(ctx context.Context, in SelfSignupInput) (*SelfSignupResult, error) {
	existing, err := s.userRepo.GetByEmail(ctx, in.Email)
	if err == nil && existing != nil {
		return nil, fmt.Errorf("SelfSignup: %w", auth.ErrEmailAlreadyRegistered)
	}

	emailPrefix := strings.ToLower(strings.Split(in.Email, "@")[0])
	sanitized := nonAlphanumeric.ReplaceAllString(emailPrefix, "")
	if sanitized == "" {
		sanitized = "user"
	}
	slug := sanitized + "-" + uuid.New().String()[:8]

	orgID := uuid.New()

	firstName := strings.Split(emailPrefix, ".")[0]
	if firstName == "" {
		firstName = "User"
	}

	user, err := auth.NewUser(orgID, in.Email, in.Password, firstName, "User")
	if err != nil {
		return nil, fmt.Errorf("SelfSignup.NewUser: %w", err)
	}
	user.Activate()
	user.VerifyToken = ""

	now := time.Now().UTC()
	org := &tenantDomain.Org{
		ID:          orgID,
		Name:        emailPrefix + "'s Org",
		Slug:        slug,
		Plan:        tenantDomain.OrgPlanFree,
		OwnerUserID: user.ID,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if err := s.orgRepo.Create(ctx, org); err != nil {
		return nil, fmt.Errorf("SelfSignup.CreateOrg: %w", err)
	}

	if err := s.userRepo.Create(ctx, user); err != nil {
		return nil, fmt.Errorf("SelfSignup.CreateUser: %w", err)
	}
	user.Roles = []auth.Role{auth.RoleAdmin}
	if err := s.userRepo.SetRoles(ctx, user.ID, user.Roles); err != nil {
		return nil, fmt.Errorf("SelfSignup.SetRoles: %w", err)
	}

	pair, err := s.issueTokenPair(ctx, user, uuid.New(), in.UserAgent, in.IPAddress)
	if err != nil {
		return nil, fmt.Errorf("SelfSignup.issueTokenPair: %w", err)
	}

	if err := s.publisher.PublishUserRegistered(ctx, user); err != nil {
		s.logger.Error("failed to publish user.registered event", "user_id", user.ID, "error", err)
	}

	return &SelfSignupResult{Pair: pair, User: user}, nil
}
