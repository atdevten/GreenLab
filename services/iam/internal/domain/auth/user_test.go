package auth

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewUser(t *testing.T) {
	tenantID := uuid.New()

	t.Run("success", func(t *testing.T) {
		user, err := NewUser(tenantID, "alice@example.com", "password123", "Alice", "Smith")
		require.NoError(t, err)
		assert.Equal(t, tenantID, user.TenantID)
		assert.Equal(t, "alice@example.com", user.Email)
		assert.Equal(t, "Alice", user.FirstName)
		assert.Equal(t, "Smith", user.LastName)
		assert.Equal(t, UserStatusPending, user.Status)
		assert.False(t, user.EmailVerified)
		assert.NotEmpty(t, user.VerifyToken)
		assert.NotEmpty(t, user.PasswordHash)
		assert.NotEqual(t, uuid.Nil, user.ID)
	})

	t.Run("empty email returns error", func(t *testing.T) {
		user, err := NewUser(tenantID, "", "password123", "Alice", "Smith")
		assert.Error(t, err)
		assert.Nil(t, user)
	})

	t.Run("password shorter than 8 chars returns error", func(t *testing.T) {
		user, err := NewUser(tenantID, "alice@example.com", "short", "Alice", "Smith")
		assert.Error(t, err)
		assert.Nil(t, user)
	})
}

func TestCheckPassword(t *testing.T) {
	tenantID := uuid.New()
	user, err := NewUser(tenantID, "alice@example.com", "correctpass", "Alice", "Smith")
	require.NoError(t, err)

	t.Run("correct password returns true", func(t *testing.T) {
		assert.True(t, user.CheckPassword("correctpass"))
	})

	t.Run("wrong password returns false", func(t *testing.T) {
		assert.False(t, user.CheckPassword("wrongpassword"))
	})
}

func TestSetPassword(t *testing.T) {
	tenantID := uuid.New()
	user, err := NewUser(tenantID, "alice@example.com", "password123", "Alice", "Smith")
	require.NoError(t, err)

	t.Run("success", func(t *testing.T) {
		err := user.SetPassword("newpassword123")
		require.NoError(t, err)
		assert.True(t, user.CheckPassword("newpassword123"))
	})

	t.Run("short password returns error", func(t *testing.T) {
		err := user.SetPassword("short")
		assert.Error(t, err)
	})
}

func TestIsActive(t *testing.T) {
	tenantID := uuid.New()

	t.Run("pending user returns false", func(t *testing.T) {
		user, err := NewUser(tenantID, "alice@example.com", "password123", "Alice", "Smith")
		require.NoError(t, err)
		assert.False(t, user.IsActive())
	})

	t.Run("active user returns true", func(t *testing.T) {
		user, err := NewUser(tenantID, "alice@example.com", "password123", "Alice", "Smith")
		require.NoError(t, err)
		user.Status = UserStatusActive
		assert.True(t, user.IsActive())
	})
}

func TestActivate(t *testing.T) {
	tenantID := uuid.New()
	user, err := NewUser(tenantID, "alice@example.com", "password123", "Alice", "Smith")
	require.NoError(t, err)

	user.Activate()

	assert.Equal(t, UserStatusActive, user.Status)
	assert.True(t, user.EmailVerified)
	assert.Empty(t, user.VerifyToken)
}

func TestDisable(t *testing.T) {
	tenantID := uuid.New()
	user, err := NewUser(tenantID, "alice@example.com", "password123", "Alice", "Smith")
	require.NoError(t, err)
	user.Status = UserStatusActive

	user.Disable()

	assert.Equal(t, UserStatusDisabled, user.Status)
}

func TestIsResetTokenValid(t *testing.T) {
	tenantID := uuid.New()
	user, err := NewUser(tenantID, "alice@example.com", "password123", "Alice", "Smith")
	require.NoError(t, err)

	t.Run("valid token returns true", func(t *testing.T) {
		token := "myresettoken"
		expiry := time.Now().UTC().Add(1 * time.Hour)
		user.SetResetToken(token, expiry)
		assert.True(t, user.IsResetTokenValid(token))
	})

	t.Run("empty token returns false", func(t *testing.T) {
		token := "myresettoken"
		expiry := time.Now().UTC().Add(1 * time.Hour)
		user.SetResetToken(token, expiry)
		assert.False(t, user.IsResetTokenValid(""))
	})

	t.Run("expired token returns false", func(t *testing.T) {
		token := "myresettoken"
		expiry := time.Now().UTC().Add(-1 * time.Hour) // already expired
		user.SetResetToken(token, expiry)
		assert.False(t, user.IsResetTokenValid(token))
	})

	t.Run("wrong token returns false", func(t *testing.T) {
		token := "myresettoken"
		expiry := time.Now().UTC().Add(1 * time.Hour)
		user.SetResetToken(token, expiry)
		assert.False(t, user.IsResetTokenValid("wrongtoken"))
	})
}

func TestHasRole(t *testing.T) {
	tenantID := uuid.New()
	user, err := NewUser(tenantID, "alice@example.com", "password123", "Alice", "Smith")
	require.NoError(t, err)
	user.Roles = []Role{RoleViewer, RoleOperator}

	t.Run("user has the role returns true", func(t *testing.T) {
		assert.True(t, user.HasRole(RoleViewer))
		assert.True(t, user.HasRole(RoleOperator))
	})

	t.Run("user does not have the role returns false", func(t *testing.T) {
		assert.False(t, user.HasRole(RoleAdmin))
	})
}
