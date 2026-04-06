package redis

import (
	"context"
	"testing"

	"github.com/go-redis/redismock/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// anyValue is a CustomMatch that accepts any value for the Redis SET command.
// Used for keys that store dynamic values like Unix timestamps.
func anyValue(expected, actual []interface{}) error {
	return nil
}

func TestSchemaDeprecationStore_SetForceDeprecated(t *testing.T) {
	rdb, mock := redismock.NewClientMock()
	store := NewSchemaDeprecationStore(rdb)
	ctx := context.Background()

	// Value is a dynamic Unix timestamp — accept any value.
	mock.CustomMatch(anyValue).ExpectSet("schema_force_deprecated:chan-1", "", forceDeprecatedTTL).SetVal("OK")

	err := store.SetForceDeprecated(ctx, "chan-1")
	require.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestSchemaDeprecationStore_SetForceDeprecated_RedisError(t *testing.T) {
	rdb, mock := redismock.NewClientMock()
	store := NewSchemaDeprecationStore(rdb)
	ctx := context.Background()

	mock.CustomMatch(anyValue).ExpectSet("schema_force_deprecated:chan-1", "", forceDeprecatedTTL).SetErr(assert.AnError)

	err := store.SetForceDeprecated(ctx, "chan-1")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "SchemaDeprecationStore.SetForceDeprecated")
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestSchemaDeprecationStore_IsForceDeprecated_True(t *testing.T) {
	rdb, mock := redismock.NewClientMock()
	store := NewSchemaDeprecationStore(rdb)
	ctx := context.Background()

	mock.ExpectGet("schema_force_deprecated:chan-1").SetVal("1712345678")

	deprecated, err := store.IsForceDeprecated(ctx, "chan-1")
	require.NoError(t, err)
	assert.True(t, deprecated)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestSchemaDeprecationStore_IsForceDeprecated_NotFound(t *testing.T) {
	rdb, mock := redismock.NewClientMock()
	store := NewSchemaDeprecationStore(rdb)
	ctx := context.Background()

	mock.ExpectGet("schema_force_deprecated:chan-1").RedisNil()

	deprecated, err := store.IsForceDeprecated(ctx, "chan-1")
	require.NoError(t, err)
	assert.False(t, deprecated)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestSchemaDeprecationStore_IsForceDeprecated_RedisError(t *testing.T) {
	rdb, mock := redismock.NewClientMock()
	store := NewSchemaDeprecationStore(rdb)
	ctx := context.Background()

	mock.ExpectGet("schema_force_deprecated:chan-1").SetErr(assert.AnError)

	deprecated, err := store.IsForceDeprecated(ctx, "chan-1")
	require.Error(t, err)
	assert.False(t, deprecated)
	assert.Contains(t, err.Error(), "SchemaDeprecationStore.IsForceDeprecated")
	assert.NoError(t, mock.ExpectationsWereMet())
}

// TestForceDeprecatedKey confirms the key format to prevent regressions.
func TestForceDeprecatedKey(t *testing.T) {
	assert.Equal(t, "schema_force_deprecated:chan-abc", forceDeprecatedKey("chan-abc"))
}
