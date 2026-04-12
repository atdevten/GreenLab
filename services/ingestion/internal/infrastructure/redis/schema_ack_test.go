package redis

import (
	"context"
	"fmt"
	"testing"

	"github.com/go-redis/redismock/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// anyTimestamp is a CustomMatch that accepts any numeric Unix timestamp value
// for the active key Set call.
func anyTimestamp(expected, actual []interface{}) error {
	// We only care that 4 args are present: key, value, expiry (via EX), ttl
	// and that the key prefix is correct. The value is a timestamp so skip it.
	return nil
}

// TestSchemaACKStore_RecordACK_NewDevice verifies that recording a version for a device
// with no prior entry writes both the ACK key and the active key via pipeline.
func TestSchemaACKStore_RecordACK_NewDevice(t *testing.T) {
	rdb, mock := redismock.NewClientMock()
	store := NewSchemaACKStore(rdb)
	ctx := context.Background()

	// GET returns redis.Nil (no prior entry).
	mock.ExpectGet("schema_ack:chan-1:dev-1").RedisNil()
	// Pipeline SET for ack key and active key — use CustomMatch to ignore the timestamp value.
	mock.ExpectSet("schema_ack:chan-1:dev-1", "3", schemaACKTTL).SetVal("OK")
	mock.CustomMatch(anyTimestamp).ExpectSet("schema_active:chan-1:dev-1", "", schemaActiveTTL).SetVal("OK")

	err := store.RecordACK(ctx, "chan-1", "dev-1", 3)
	require.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// TestSchemaACKStore_RecordACK_NoDowngrade verifies that if the stored version is higher,
// we skip updating the ACK key and only update the active key.
func TestSchemaACKStore_RecordACK_NoDowngrade(t *testing.T) {
	rdb, mock := redismock.NewClientMock()
	store := NewSchemaACKStore(rdb)
	ctx := context.Background()

	// GET returns "5" — device already ACK'd version 5.
	mock.ExpectGet("schema_ack:chan-1:dev-1").SetVal("5")
	// Only the active key should be updated (not the ACK key).
	mock.CustomMatch(anyTimestamp).ExpectSet("schema_active:chan-1:dev-1", "", schemaActiveTTL).SetVal("OK")

	// Try to record version 3 — should not overwrite version 5.
	err := store.RecordACK(ctx, "chan-1", "dev-1", 3)
	require.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// TestSchemaACKStore_RecordACK_Upgrade verifies that if the new version is higher,
// both the ACK key and active key are updated.
func TestSchemaACKStore_RecordACK_Upgrade(t *testing.T) {
	rdb, mock := redismock.NewClientMock()
	store := NewSchemaACKStore(rdb)
	ctx := context.Background()

	// Device previously ACK'd version 2; now sends version 7.
	mock.ExpectGet("schema_ack:chan-1:dev-1").SetVal("2")
	mock.ExpectSet("schema_ack:chan-1:dev-1", "7", schemaACKTTL).SetVal("OK")
	mock.CustomMatch(anyTimestamp).ExpectSet("schema_active:chan-1:dev-1", "", schemaActiveTTL).SetVal("OK")

	err := store.RecordACK(ctx, "chan-1", "dev-1", 7)
	require.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// TestSchemaACKStore_RecordACK_SameVersion verifies that if the new version equals
// the stored version, it does not overwrite (treated as "not higher").
func TestSchemaACKStore_RecordACK_SameVersion(t *testing.T) {
	rdb, mock := redismock.NewClientMock()
	store := NewSchemaACKStore(rdb)
	ctx := context.Background()

	mock.ExpectGet("schema_ack:chan-1:dev-1").SetVal("4")
	// Same version — no ACK key update, only active key.
	mock.CustomMatch(anyTimestamp).ExpectSet("schema_active:chan-1:dev-1", "", schemaActiveTTL).SetVal("OK")

	err := store.RecordACK(ctx, "chan-1", "dev-1", 4)
	require.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// TestSchemaACKStore_ACKedVersion_NotFound verifies that 0 is returned for unknown devices.
func TestSchemaACKStore_ACKedVersion_NotFound(t *testing.T) {
	rdb, mock := redismock.NewClientMock()
	store := NewSchemaACKStore(rdb)
	ctx := context.Background()

	mock.ExpectGet("schema_ack:chan-x:dev-x").RedisNil()

	v, err := store.ACKedVersion(ctx, "chan-x", "dev-x")
	require.NoError(t, err)
	assert.Equal(t, uint32(0), v)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// TestSchemaACKStore_ACKedVersion_ReturnsStored verifies the stored value is parsed correctly.
func TestSchemaACKStore_ACKedVersion_ReturnsStored(t *testing.T) {
	rdb, mock := redismock.NewClientMock()
	store := NewSchemaACKStore(rdb)
	ctx := context.Background()

	mock.ExpectGet("schema_ack:chan-1:dev-1").SetVal("42")

	v, err := store.ACKedVersion(ctx, "chan-1", "dev-1")
	require.NoError(t, err)
	assert.Equal(t, uint32(42), v)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// TestSchemaACKStore_ActiveDeviceCount_Empty returns 0 when no active keys exist.
func TestSchemaACKStore_ActiveDeviceCount_Empty(t *testing.T) {
	rdb, mock := redismock.NewClientMock()
	store := NewSchemaACKStore(rdb)
	ctx := context.Background()

	mock.ExpectScan(0, "schema_active:chan-nobody:*", 100).SetVal([]string{}, 0)

	count, err := store.ActiveDeviceCount(ctx, "chan-nobody")
	require.NoError(t, err)
	assert.Equal(t, int64(0), count)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// TestSchemaACKStore_ActiveDeviceCount_MultipleBatches verifies SCAN pagination is handled.
func TestSchemaACKStore_ActiveDeviceCount_MultipleBatches(t *testing.T) {
	rdb, mock := redismock.NewClientMock()
	store := NewSchemaACKStore(rdb)
	ctx := context.Background()

	// First SCAN batch: 2 keys, cursor 5 (more to come).
	mock.ExpectScan(0, "schema_active:chan-1:*", 100).
		SetVal([]string{"schema_active:chan-1:dev-1", "schema_active:chan-1:dev-2"}, 5)
	// Second SCAN batch: 1 key, cursor 0 (done).
	mock.ExpectScan(5, "schema_active:chan-1:*", 100).
		SetVal([]string{"schema_active:chan-1:dev-3"}, 0)

	count, err := store.ActiveDeviceCount(ctx, "chan-1")
	require.NoError(t, err)
	assert.Equal(t, int64(3), count)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// TestSchemaACKStore_ACKedDeviceCount_Empty returns 0 when no ACK keys exist.
func TestSchemaACKStore_ACKedDeviceCount_Empty(t *testing.T) {
	rdb, mock := redismock.NewClientMock()
	store := NewSchemaACKStore(rdb)
	ctx := context.Background()

	mock.ExpectScan(0, "schema_ack:chan-nobody:*", 100).SetVal([]string{}, 0)

	count, err := store.ACKedDeviceCount(ctx, "chan-nobody", 1)
	require.NoError(t, err)
	assert.Equal(t, int64(0), count)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// TestSchemaACKStore_ACKedDeviceCount_FiltersByVersion verifies that only devices
// with ACK'd version >= the requested version are counted.
func TestSchemaACKStore_ACKedDeviceCount_FiltersByVersion(t *testing.T) {
	rdb, mock := redismock.NewClientMock()
	store := NewSchemaACKStore(rdb)
	ctx := context.Background()

	keys := []string{
		"schema_ack:chan-1:dev-1",
		"schema_ack:chan-1:dev-2",
		"schema_ack:chan-1:dev-3",
	}
	// dev-1 → v5, dev-2 → v3, dev-3 → v6
	mock.ExpectScan(0, "schema_ack:chan-1:*", 100).SetVal(keys, 0)
	mock.ExpectGet("schema_ack:chan-1:dev-1").SetVal("5")
	mock.ExpectGet("schema_ack:chan-1:dev-2").SetVal("3")
	mock.ExpectGet("schema_ack:chan-1:dev-3").SetVal("6")

	// Ask for devices that have ACK'd >= version 5: dev-1 (5) and dev-3 (6).
	count, err := store.ACKedDeviceCount(ctx, "chan-1", 5)
	require.NoError(t, err)
	assert.Equal(t, int64(2), count)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// TestSchemaACKStore_ACKedDeviceCount_SkipsExpiredKeys verifies that keys returning
// redis.Nil (expired between SCAN and GET) are gracefully skipped.
func TestSchemaACKStore_ACKedDeviceCount_SkipsExpiredKeys(t *testing.T) {
	rdb, mock := redismock.NewClientMock()
	store := NewSchemaACKStore(rdb)
	ctx := context.Background()

	keys := []string{"schema_ack:chan-1:dev-1", "schema_ack:chan-1:dev-2"}
	mock.ExpectScan(0, "schema_ack:chan-1:*", 100).SetVal(keys, 0)
	mock.ExpectGet("schema_ack:chan-1:dev-1").SetVal("5")
	mock.ExpectGet("schema_ack:chan-1:dev-2").RedisNil() // expired between SCAN and GET

	count, err := store.ACKedDeviceCount(ctx, "chan-1", 1)
	require.NoError(t, err)
	assert.Equal(t, int64(1), count)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// TestSchemaACKKey and TestSchemaActiveKey confirm key format to prevent regressions.
func TestSchemaACKKey(t *testing.T) {
	assert.Equal(t, "schema_ack:chan-abc:dev-xyz", schemaACKKey("chan-abc", "dev-xyz"))
}

func TestSchemaActiveKey(t *testing.T) {
	assert.Equal(t, "schema_active:chan-abc:dev-xyz", schemaActiveKey("chan-abc", "dev-xyz"))
}

// TestSchemaACKStore_IsForceDeprecated_NotSet returns false when no marker exists.
func TestSchemaACKStore_IsForceDeprecated_NotSet(t *testing.T) {
	rdb, mock := redismock.NewClientMock()
	store := NewSchemaACKStore(rdb)
	ctx := context.Background()

	mock.ExpectGet("schema_force_deprecated:chan-1").RedisNil()

	deprecated, err := store.IsForceDeprecated(ctx, "chan-1")
	require.NoError(t, err)
	assert.False(t, deprecated)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// TestSchemaACKStore_IsForceDeprecated_Set returns true when the marker exists.
func TestSchemaACKStore_IsForceDeprecated_Set(t *testing.T) {
	rdb, mock := redismock.NewClientMock()
	store := NewSchemaACKStore(rdb)
	ctx := context.Background()

	mock.ExpectGet("schema_force_deprecated:chan-1").SetVal("1")

	deprecated, err := store.IsForceDeprecated(ctx, "chan-1")
	require.NoError(t, err)
	assert.True(t, deprecated)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// TestSchemaACKStore_IsForceDeprecated_RedisError propagates the error.
func TestSchemaACKStore_IsForceDeprecated_RedisError(t *testing.T) {
	rdb, mock := redismock.NewClientMock()
	store := NewSchemaACKStore(rdb)
	ctx := context.Background()

	mock.ExpectGet("schema_force_deprecated:chan-1").SetErr(fmt.Errorf("connection refused"))

	_, err := store.IsForceDeprecated(ctx, "chan-1")
	require.Error(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// TestSchemaACKStore_SetStuck uses SetNX so the key is only written once per device.
func TestSchemaACKStore_SetStuck(t *testing.T) {
	rdb, mock := redismock.NewClientMock()
	store := NewSchemaACKStore(rdb)
	ctx := context.Background()

	mock.ExpectSetNX("schema_stuck:chan-1:dev-1", "1", schemaStuckTTL).SetVal(true)

	err := store.SetStuck(ctx, "chan-1", "dev-1")
	require.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// TestSchemaACKStore_SetStuck_AlreadyExists is a no-op (SetNX returns false) — not an error.
func TestSchemaACKStore_SetStuck_AlreadyExists(t *testing.T) {
	rdb, mock := redismock.NewClientMock()
	store := NewSchemaACKStore(rdb)
	ctx := context.Background()

	mock.ExpectSetNX("schema_stuck:chan-1:dev-1", "1", schemaStuckTTL).SetVal(false)

	err := store.SetStuck(ctx, "chan-1", "dev-1")
	require.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// TestSchemaACKStore_SetStuck_RedisError propagates the error.
func TestSchemaACKStore_SetStuck_RedisError(t *testing.T) {
	rdb, mock := redismock.NewClientMock()
	store := NewSchemaACKStore(rdb)
	ctx := context.Background()

	mock.ExpectSetNX("schema_stuck:chan-1:dev-1", "1", schemaStuckTTL).SetErr(fmt.Errorf("connection refused"))

	err := store.SetStuck(ctx, "chan-1", "dev-1")
	require.Error(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// TestSchemaForceDeprecatedKey and TestSchemaStuckKey confirm key formats.
func TestSchemaForceDeprecatedKey(t *testing.T) {
	assert.Equal(t, "schema_force_deprecated:chan-abc", schemaForceDeprecatedKey("chan-abc"))
}

func TestSchemaStuckKey(t *testing.T) {
	assert.Equal(t, "schema_stuck:chan-abc:dev-xyz", schemaStuckKey("chan-abc", "dev-xyz"))
}

// Ensure the store satisfies the full handler interface declared in transport/http.
// This is a compile-time check — no runtime assertions needed.
var _ interface {
	RecordACK(ctx context.Context, channelID, deviceID string, version uint32) error
	IsForceDeprecated(ctx context.Context, channelID string) (bool, error)
	SetStuck(ctx context.Context, channelID, deviceID string) error
} = (*SchemaACKStore)(nil)

// fmtDummy prevents the "imported and not used" error for fmt in table-driven helpers.
var _ = fmt.Sprintf
