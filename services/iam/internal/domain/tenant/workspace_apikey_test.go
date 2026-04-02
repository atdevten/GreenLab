package tenant

import (
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewWorkspaceAPIKey_Success_Read(t *testing.T) {
	wsID := uuid.New()
	key, plainKey, err := NewWorkspaceAPIKey(wsID, "dashboard", "read")
	require.NoError(t, err)
	assert.NotNil(t, key)
	assert.NotEmpty(t, plainKey)
	assert.True(t, strings.HasPrefix(plainKey, "wsk_"), "raw key must start with wsk_")
	assert.Len(t, plainKey, 68) // "wsk_" + 64 hex chars
	assert.Equal(t, plainKey[:8], key.KeyPrefix)
	assert.NotEmpty(t, key.KeyHash)
	assert.Equal(t, "read", key.Scope)
	assert.Equal(t, "dashboard", key.Name)
	assert.Equal(t, wsID, key.WorkspaceID)
	assert.NotEqual(t, uuid.Nil, key.ID)
	assert.Nil(t, key.LastUsedAt)
	assert.Nil(t, key.RevokedAt)
}

func TestNewWorkspaceAPIKey_Success_Write(t *testing.T) {
	wsID := uuid.New()
	key, plainKey, err := NewWorkspaceAPIKey(wsID, "ingest", "write")
	require.NoError(t, err)
	assert.Equal(t, "write", key.Scope)
	assert.NotEmpty(t, plainKey)
}

func TestNewWorkspaceAPIKey_EmptyName(t *testing.T) {
	wsID := uuid.New()
	key, plainKey, err := NewWorkspaceAPIKey(wsID, "", "read")
	assert.Error(t, err)
	assert.ErrorIs(t, err, ErrInvalidName)
	assert.Nil(t, key)
	assert.Empty(t, plainKey)
}

func TestNewWorkspaceAPIKey_InvalidScope(t *testing.T) {
	wsID := uuid.New()
	key, plainKey, err := NewWorkspaceAPIKey(wsID, "dashboard", "admin")
	assert.Error(t, err)
	assert.ErrorIs(t, err, ErrInvalidScope)
	assert.Nil(t, key)
	assert.Empty(t, plainKey)
}

func TestNewWorkspaceAPIKey_KeyHashDiffersFromPlain(t *testing.T) {
	wsID := uuid.New()
	key, plainKey, err := NewWorkspaceAPIKey(wsID, "test", "read")
	require.NoError(t, err)
	assert.NotEqual(t, plainKey, key.KeyHash)
}

func TestNewWorkspaceAPIKey_UniqueKeysEachCall(t *testing.T) {
	wsID := uuid.New()
	_, key1, err1 := NewWorkspaceAPIKey(wsID, "a", "read")
	_, key2, err2 := NewWorkspaceAPIKey(wsID, "b", "read")
	require.NoError(t, err1)
	require.NoError(t, err2)
	assert.NotEqual(t, key1, key2)
}
