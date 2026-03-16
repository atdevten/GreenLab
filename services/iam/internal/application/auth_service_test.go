package application

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/greenlab/iam/internal/domain/auth"
	mockauth "github.com/greenlab/iam/internal/mocks/auth"
	mocktenant "github.com/greenlab/iam/internal/mocks/tenant"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// newTestAuthService creates an AuthService with all mocked dependencies.
func newTestAuthService(t *testing.T) (
	*AuthService,
	*mockauth.MockUserRepository,
	*mockauth.MockTokenRepository,
	*mockauth.MockCacheRepository,
	*mockauth.MockEventPublisher,
	*mocktenant.MockOrgRepository,
) {
	t.Helper()

	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	userRepo := mockauth.NewMockUserRepository(t)
	tokenRepo := mockauth.NewMockTokenRepository(t)
	cache := mockauth.NewMockCacheRepository(t)
	publisher := mockauth.NewMockEventPublisher(t)
	orgRepo := mocktenant.NewMockOrgRepository(t)

	svc := NewAuthService(userRepo, tokenRepo, cache, publisher, orgRepo, privateKey, &privateKey.PublicKey, "test-issuer")
	return svc, userRepo, tokenRepo, cache, publisher, orgRepo
}

// newActiveUser creates a pre-activated user with a known password for tests.
func newActiveUser(t *testing.T, password string) *auth.User {
	t.Helper()
	tenantID := uuid.New()
	user, err := auth.NewUser(tenantID, "alice@example.com", password, "Alice", "Smith")
	require.NoError(t, err)
	user.Activate()
	return user
}

// makeActiveRefreshToken returns a valid stored token for a given user.
func makeActiveRefreshToken(userID uuid.UUID) *auth.RefreshToken {
	expiry := time.Now().UTC().Add(7 * 24 * time.Hour)
	return &auth.RefreshToken{
		ID:        uuid.New(),
		UserID:    userID,
		TokenHash: "somehash",
		Family:    uuid.New(),
		ExpiresAt: expiry,
		Revoked:   false,
	}
}

// --- Register ---

func TestRegister_Success(t *testing.T) {
	svc, userRepo, _, _, publisher, _ := newTestAuthService(t)
	ctx := context.Background()

	tenantID := uuid.New()

	userRepo.On("GetByEmail", ctx, "alice@example.com").Return(nil, auth.ErrUserNotFound)
	userRepo.On("Create", ctx, mock.AnythingOfType("*auth.User")).Return(nil)
	userRepo.On("SetRoles", ctx, mock.AnythingOfType("uuid.UUID"), mock.AnythingOfType("[]auth.Role")).Return(nil)
	publisher.On("PublishUserRegistered", ctx, mock.AnythingOfType("*auth.User")).Return(nil)

	user, err := svc.Register(ctx, RegisterInput{
		TenantID:  tenantID.String(),
		Email:     "alice@example.com",
		Password:  "securepassword",
		FirstName: "Alice",
		LastName:  "Smith",
	})
	require.NoError(t, err)
	assert.NotNil(t, user)
	assert.Equal(t, "alice@example.com", user.Email)
}

func TestRegister_EmailAlreadyExists(t *testing.T) {
	svc, userRepo, _, _, _, _ := newTestAuthService(t)
	ctx := context.Background()

	existing := &auth.User{ID: uuid.New(), Email: "alice@example.com"}
	userRepo.On("GetByEmail", ctx, "alice@example.com").Return(existing, nil)

	user, err := svc.Register(ctx, RegisterInput{
		TenantID:  uuid.New().String(),
		Email:     "alice@example.com",
		Password:  "securepassword",
		FirstName: "Alice",
		LastName:  "Smith",
	})
	assert.Error(t, err)
	assert.Nil(t, user)
	assert.ErrorIs(t, err, auth.ErrEmailAlreadyRegistered)
}

func TestRegister_InvalidTenantID(t *testing.T) {
	svc, _, _, _, _, _ := newTestAuthService(t)
	ctx := context.Background()

	user, err := svc.Register(ctx, RegisterInput{
		TenantID:  "not-a-uuid",
		Email:     "alice@example.com",
		Password:  "securepassword",
		FirstName: "Alice",
		LastName:  "Smith",
	})
	assert.Error(t, err)
	assert.Nil(t, user)
}

// --- Login ---

func TestLogin_Success(t *testing.T) {
	svc, userRepo, tokenRepo, _, publisher, _ := newTestAuthService(t)
	ctx := context.Background()

	password := "securepassword"
	user := newActiveUser(t, password)
	roles := []auth.Role{auth.RoleViewer}

	userRepo.On("GetByEmail", ctx, user.Email).Return(user, nil)
	userRepo.On("GetRoles", ctx, user.ID).Return(roles, nil)
	tokenRepo.On("SaveRefreshToken", ctx, mock.AnythingOfType("*auth.RefreshToken")).Return(nil)
	publisher.On("PublishUserLoggedIn", ctx, mock.AnythingOfType("*auth.User"), "127.0.0.1").Return(nil)

	pair, err := svc.Login(ctx, LoginInput{
		Email:     user.Email,
		Password:  password,
		UserAgent: "test-agent",
		IPAddress: "127.0.0.1",
	})
	require.NoError(t, err)
	assert.NotNil(t, pair)
	assert.NotEmpty(t, pair.AccessToken)
	assert.NotEmpty(t, pair.RefreshToken)
}

func TestLogin_UserNotFound(t *testing.T) {
	svc, userRepo, _, _, _, _ := newTestAuthService(t)
	ctx := context.Background()

	userRepo.On("GetByEmail", ctx, "unknown@example.com").Return(nil, auth.ErrUserNotFound)

	pair, err := svc.Login(ctx, LoginInput{Email: "unknown@example.com", Password: "somepassword"})
	assert.Error(t, err)
	assert.Nil(t, pair)
	assert.ErrorIs(t, err, auth.ErrInvalidCredentials)
}

func TestLogin_WrongPassword(t *testing.T) {
	svc, userRepo, _, _, _, _ := newTestAuthService(t)
	ctx := context.Background()

	user := newActiveUser(t, "correctpassword")
	userRepo.On("GetByEmail", ctx, user.Email).Return(user, nil)

	pair, err := svc.Login(ctx, LoginInput{Email: user.Email, Password: "wrongpassword"})
	assert.Error(t, err)
	assert.Nil(t, pair)
	assert.ErrorIs(t, err, auth.ErrInvalidCredentials)
}

func TestLogin_AccountNotActive(t *testing.T) {
	svc, userRepo, _, _, _, _ := newTestAuthService(t)
	ctx := context.Background()

	password := "securepassword"
	pendingUser, err := auth.NewUser(uuid.New(), "pending@example.com", password, "Pending", "User")
	require.NoError(t, err)

	userRepo.On("GetByEmail", ctx, pendingUser.Email).Return(pendingUser, nil)

	pair, err := svc.Login(ctx, LoginInput{Email: pendingUser.Email, Password: password})
	assert.Error(t, err)
	assert.Nil(t, pair)
	assert.ErrorIs(t, err, auth.ErrAccountIsNotActive)
}

// --- ForgotPassword ---

func TestForgotPassword_Success(t *testing.T) {
	svc, userRepo, _, _, _, _ := newTestAuthService(t)
	ctx := context.Background()

	user := newActiveUser(t, "password")
	userRepo.On("GetByEmail", ctx, user.Email).Return(user, nil)
	userRepo.On("Update", ctx, mock.AnythingOfType("*auth.User")).Return(nil)

	err := svc.ForgotPassword(ctx, user.Email)
	assert.NoError(t, err)
}

func TestForgotPassword_UserNotFound(t *testing.T) {
	svc, userRepo, _, _, _, _ := newTestAuthService(t)
	ctx := context.Background()

	userRepo.On("GetByEmail", ctx, "ghost@example.com").Return(nil, auth.ErrUserNotFound)

	err := svc.ForgotPassword(ctx, "ghost@example.com")
	assert.NoError(t, err) // no enumeration — always nil
}

func TestForgotPassword_DBError(t *testing.T) {
	svc, userRepo, _, _, _, _ := newTestAuthService(t)
	ctx := context.Background()

	userRepo.On("GetByEmail", ctx, "alice@example.com").Return(nil, errors.New("database unavailable"))

	err := svc.ForgotPassword(ctx, "alice@example.com")
	assert.Error(t, err)
}

// --- ResetPassword ---

func TestResetPassword_Success(t *testing.T) {
	svc, userRepo, tokenRepo, _, publisher, _ := newTestAuthService(t)
	ctx := context.Background()

	rawToken := "valid-reset-token"
	tokenHash := hashToken(rawToken)
	expiry := time.Now().UTC().Add(1 * time.Hour)

	user := newActiveUser(t, "oldpassword")
	user.SetResetToken(tokenHash, expiry)

	userRepo.On("GetByResetToken", ctx, tokenHash).Return(user, nil)
	userRepo.On("Update", ctx, mock.AnythingOfType("*auth.User")).Return(nil)
	tokenRepo.On("RevokeAllForUser", ctx, user.ID).Return(nil)
	publisher.On("PublishPasswordChanged", ctx, user.ID).Return(nil)

	err := svc.ResetPassword(ctx, rawToken, "newpassword123")
	require.NoError(t, err)
}

func TestResetPassword_InvalidToken(t *testing.T) {
	svc, userRepo, _, _, _, _ := newTestAuthService(t)
	ctx := context.Background()

	userRepo.On("GetByResetToken", ctx, mock.AnythingOfType("string")).Return(nil, auth.ErrUserNotFound)

	err := svc.ResetPassword(ctx, "badtoken", "newpassword123")
	assert.Error(t, err)
	assert.ErrorIs(t, err, auth.ErrInvalidResetToken)
}

// --- VerifyEmail ---

func TestVerifyEmail_Success(t *testing.T) {
	svc, userRepo, _, _, publisher, _ := newTestAuthService(t)
	ctx := context.Background()

	rawToken := "verify-token"
	tokenHash := hashToken(rawToken)

	user, err := auth.NewUser(uuid.New(), "user@example.com", "password123", "Test", "User")
	require.NoError(t, err)

	userRepo.On("GetByVerifyToken", ctx, tokenHash).Return(user, nil)
	userRepo.On("Update", ctx, mock.AnythingOfType("*auth.User")).Return(nil)
	publisher.On("PublishEmailVerified", ctx, user.ID).Return(nil)

	err = svc.VerifyEmail(ctx, rawToken)
	require.NoError(t, err)
}

func TestVerifyEmail_AlreadyVerified(t *testing.T) {
	svc, userRepo, _, _, _, _ := newTestAuthService(t)
	ctx := context.Background()

	rawToken := "verify-token"
	tokenHash := hashToken(rawToken)

	user, err := auth.NewUser(uuid.New(), "user@example.com", "password123", "Test", "User")
	require.NoError(t, err)
	user.EmailVerified = true

	userRepo.On("GetByVerifyToken", ctx, tokenHash).Return(user, nil)

	err = svc.VerifyEmail(ctx, rawToken)
	require.NoError(t, err)
}

func TestVerifyEmail_InvalidToken(t *testing.T) {
	svc, userRepo, _, _, _, _ := newTestAuthService(t)
	ctx := context.Background()

	userRepo.On("GetByVerifyToken", ctx, mock.AnythingOfType("string")).Return(nil, auth.ErrUserNotFound)

	err := svc.VerifyEmail(ctx, "invalidtoken")
	assert.Error(t, err)
	assert.ErrorIs(t, err, auth.ErrInvalidToken)
}

// --- Logout ---

func TestLogout_AlwaysReturnsNil(t *testing.T) {
	svc, _, tokenRepo, cache, _, _ := newTestAuthService(t)
	ctx := context.Background()

	tokenRepo.On("RevokeRefreshToken", ctx, mock.AnythingOfType("string")).Return(nil)
	cache.On("BlacklistToken", ctx, "some-jti", 900).Return(nil)

	err := svc.Logout(ctx, "someRefreshToken", "some-jti", 900)
	assert.NoError(t, err)
}

func TestLogout_EmptyTokens(t *testing.T) {
	svc, _, _, _, _, _ := newTestAuthService(t)
	ctx := context.Background()

	err := svc.Logout(ctx, "", "", 0)
	assert.NoError(t, err)
}

// --- Refresh ---

func TestRefresh_Success(t *testing.T) {
	svc, userRepo, tokenRepo, _, _, _ := newTestAuthService(t)
	ctx := context.Background()

	user := newActiveUser(t, "securepassword")
	roles := []auth.Role{auth.RoleViewer}
	stored := makeActiveRefreshToken(user.ID)

	tokenRepo.On("GetAndRevokeRefreshToken", ctx, mock.AnythingOfType("string")).Return(stored, nil)
	userRepo.On("GetByID", ctx, user.ID).Return(user, nil)
	userRepo.On("GetRoles", ctx, user.ID).Return(roles, nil)
	tokenRepo.On("SaveRefreshToken", ctx, mock.AnythingOfType("*auth.RefreshToken")).Return(nil)

	pair, err := svc.Refresh(ctx, RefreshInput{
		RefreshToken: "raw-token-value",
		UserAgent:    "test-agent",
		IPAddress:    "127.0.0.1",
	})
	require.NoError(t, err)
	assert.NotNil(t, pair)
	assert.NotEmpty(t, pair.AccessToken)
	assert.NotEmpty(t, pair.RefreshToken)
}

func TestRefresh_TokenNotFound(t *testing.T) {
	svc, _, tokenRepo, _, _, _ := newTestAuthService(t)
	ctx := context.Background()

	tokenRepo.On("GetAndRevokeRefreshToken", ctx, mock.AnythingOfType("string")).
		Return(nil, auth.ErrTokenNotFound)

	pair, err := svc.Refresh(ctx, RefreshInput{RefreshToken: "stale-token"})
	assert.Error(t, err)
	assert.Nil(t, pair)
	assert.ErrorIs(t, err, auth.ErrInvalidToken)
}

func TestRefresh_AccountNotActive(t *testing.T) {
	svc, userRepo, tokenRepo, _, _, _ := newTestAuthService(t)
	ctx := context.Background()

	inactiveUser, err := auth.NewUser(uuid.New(), "pending@example.com", "securepassword", "Pending", "User")
	require.NoError(t, err)

	stored := makeActiveRefreshToken(inactiveUser.ID)

	tokenRepo.On("GetAndRevokeRefreshToken", ctx, mock.AnythingOfType("string")).Return(stored, nil)
	userRepo.On("GetByID", ctx, inactiveUser.ID).Return(inactiveUser, nil)

	pair, err := svc.Refresh(ctx, RefreshInput{RefreshToken: "raw-token-value"})
	assert.Error(t, err)
	assert.Nil(t, pair)
	assert.ErrorIs(t, err, auth.ErrAccountIsNotActive)
}

// --- GetMe ---

func TestGetMe_Success(t *testing.T) {
	svc, userRepo, _, _, _, _ := newTestAuthService(t)
	ctx := context.Background()

	user := newActiveUser(t, "password")
	roles := []auth.Role{auth.RoleViewer}

	userRepo.On("GetByID", ctx, user.ID).Return(user, nil)
	userRepo.On("GetRoles", ctx, user.ID).Return(roles, nil)

	result, err := svc.GetMe(ctx, user.ID.String())
	require.NoError(t, err)
	assert.Equal(t, user, result)
	assert.Equal(t, roles, result.Roles)
}

func TestGetMe_NotFound(t *testing.T) {
	svc, userRepo, _, _, _, _ := newTestAuthService(t)
	ctx := context.Background()

	id := uuid.New()
	userRepo.On("GetByID", ctx, id).Return(nil, auth.ErrUserNotFound)

	result, err := svc.GetMe(ctx, id.String())
	assert.Error(t, err)
	assert.Nil(t, result)
}

func TestGetMe_InvalidUUID(t *testing.T) {
	svc, _, _, _, _, _ := newTestAuthService(t)
	ctx := context.Background()

	result, err := svc.GetMe(ctx, "not-a-uuid")
	assert.Error(t, err)
	assert.Nil(t, result)
}

// --- UpdateMe ---

func TestUpdateMe_Success(t *testing.T) {
	svc, userRepo, _, _, _, _ := newTestAuthService(t)
	ctx := context.Background()

	user := newActiveUser(t, "password")
	userRepo.On("GetByID", ctx, user.ID).Return(user, nil)
	userRepo.On("Update", ctx, mock.AnythingOfType("*auth.User")).Return(nil)

	result, err := svc.UpdateMe(ctx, user.ID.String(), UpdateMeInput{FirstName: "Bob"})
	require.NoError(t, err)
	assert.Equal(t, "Bob", result.FirstName)
}

func TestUpdateMe_NotFound(t *testing.T) {
	svc, userRepo, _, _, _, _ := newTestAuthService(t)
	ctx := context.Background()

	id := uuid.New()
	userRepo.On("GetByID", ctx, id).Return(nil, auth.ErrUserNotFound)

	result, err := svc.UpdateMe(ctx, id.String(), UpdateMeInput{FirstName: "Bob"})
	assert.Error(t, err)
	assert.Nil(t, result)
}

// --- ChangePassword ---

func TestChangePassword_Success(t *testing.T) {
	svc, userRepo, tokenRepo, _, publisher, _ := newTestAuthService(t)
	ctx := context.Background()

	currentPassword := "currentpass"
	user := newActiveUser(t, currentPassword)

	userRepo.On("GetByID", ctx, user.ID).Return(user, nil)
	userRepo.On("Update", ctx, mock.AnythingOfType("*auth.User")).Return(nil)
	tokenRepo.On("RevokeAllForUser", ctx, user.ID).Return(nil)
	publisher.On("PublishPasswordChanged", ctx, user.ID).Return(nil)

	err := svc.ChangePassword(ctx, user.ID.String(), currentPassword, "newpassword123")
	require.NoError(t, err)
}

func TestChangePassword_WrongPassword(t *testing.T) {
	svc, userRepo, _, _, _, _ := newTestAuthService(t)
	ctx := context.Background()

	user := newActiveUser(t, "correctpassword")
	userRepo.On("GetByID", ctx, user.ID).Return(user, nil)

	err := svc.ChangePassword(ctx, user.ID.String(), "wrongpassword", "newpassword123")
	assert.Error(t, err)
	assert.ErrorIs(t, err, auth.ErrInvalidPassword)
}

func TestChangePassword_InvalidUUID(t *testing.T) {
	svc, _, _, _, _, _ := newTestAuthService(t)
	ctx := context.Background()

	err := svc.ChangePassword(ctx, "not-a-uuid", "password", "newpassword")
	assert.Error(t, err)
}

// --- SelfSignup ---

func TestSelfSignup_Success(t *testing.T) {
	svc, userRepo, tokenRepo, _, publisher, orgRepo := newTestAuthService(t)
	ctx := context.Background()

	userRepo.On("GetByEmail", ctx, "alice@example.com").Return(nil, auth.ErrUserNotFound)
	orgRepo.On("Create", ctx, mock.AnythingOfType("*tenant.Org")).Return(nil)
	userRepo.On("Create", ctx, mock.AnythingOfType("*auth.User")).Return(nil)
	userRepo.On("SetRoles", ctx, mock.AnythingOfType("uuid.UUID"), mock.AnythingOfType("[]auth.Role")).Return(nil)
	tokenRepo.On("SaveRefreshToken", ctx, mock.AnythingOfType("*auth.RefreshToken")).Return(nil)
	publisher.On("PublishUserRegistered", ctx, mock.AnythingOfType("*auth.User")).Return(nil)

	result, err := svc.SelfSignup(ctx, SelfSignupInput{
		Email:     "alice@example.com",
		Password:  "securepassword",
		UserAgent: "test-agent",
		IPAddress: "127.0.0.1",
	})
	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.NotNil(t, result.Pair)
	assert.NotNil(t, result.User)
	assert.NotEmpty(t, result.Pair.AccessToken)
	assert.NotEmpty(t, result.Pair.RefreshToken)
}

func TestSelfSignup_EmailAlreadyExists(t *testing.T) {
	svc, userRepo, _, _, _, _ := newTestAuthService(t)
	ctx := context.Background()

	existing := &auth.User{ID: uuid.New(), Email: "alice@example.com"}
	userRepo.On("GetByEmail", ctx, "alice@example.com").Return(existing, nil)

	result, err := svc.SelfSignup(ctx, SelfSignupInput{
		Email:    "alice@example.com",
		Password: "securepassword",
	})
	assert.Error(t, err)
	assert.Nil(t, result)
	assert.ErrorIs(t, err, auth.ErrEmailAlreadyRegistered)
}
