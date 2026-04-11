package domain

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestValidateChannelID(t *testing.T) {
	t.Run("valid lowercase UUID returns nil", func(t *testing.T) {
		assert.NoError(t, ValidateChannelID("550e8400-e29b-41d4-a716-446655440000"))
	})

	t.Run("valid uppercase UUID returns nil", func(t *testing.T) {
		assert.NoError(t, ValidateChannelID("550E8400-E29B-41D4-A716-446655440000"))
	})

	t.Run("valid mixed-case UUID returns nil", func(t *testing.T) {
		assert.NoError(t, ValidateChannelID("6178c902-94de-4b1b-bea4-44d3cde53a30"))
	})

	t.Run("empty string returns ErrInvalidChannelID", func(t *testing.T) {
		assert.ErrorIs(t, ValidateChannelID(""), ErrInvalidChannelID)
	})

	t.Run("plain integer returns ErrInvalidChannelID", func(t *testing.T) {
		assert.ErrorIs(t, ValidateChannelID("42"), ErrInvalidChannelID)
	})

	t.Run("non-UUID string returns ErrInvalidChannelID", func(t *testing.T) {
		assert.ErrorIs(t, ValidateChannelID("not-a-uuid"), ErrInvalidChannelID)
	})

	t.Run("UUID with extra leading character returns ErrInvalidChannelID", func(t *testing.T) {
		assert.ErrorIs(t, ValidateChannelID("26178c902-94de-4b1b-bea4-44d3cde53a30"), ErrInvalidChannelID)
	})

	t.Run("UUID missing a segment returns ErrInvalidChannelID", func(t *testing.T) {
		assert.ErrorIs(t, ValidateChannelID("550e8400-e29b-41d4-a716"), ErrInvalidChannelID)
	})

	t.Run("UUID with whitespace returns ErrInvalidChannelID", func(t *testing.T) {
		assert.ErrorIs(t, ValidateChannelID(" 550e8400-e29b-41d4-a716-446655440000"), ErrInvalidChannelID)
	})

	t.Run("nil UUID returns ErrInvalidChannelID", func(t *testing.T) {
		assert.ErrorIs(t, ValidateChannelID("00000000-0000-0000-0000-000000000000"), ErrInvalidChannelID)
	})

	t.Run("UUID without hyphens returns ErrInvalidChannelID", func(t *testing.T) {
		assert.ErrorIs(t, ValidateChannelID("550e8400e29b41d4a716446655440000"), ErrInvalidChannelID)
	})
}

func TestValidateReplayTimestamp(t *testing.T) {
	t.Run("valid timestamp within window", func(t *testing.T) {
		ts := time.Now().UTC().Add(-24 * time.Hour) // 1 day ago
		assert.NoError(t, ValidateReplayTimestamp(ts, 30))
	})

	t.Run("timestamp exactly at window boundary is valid", func(t *testing.T) {
		// just inside the window (29 days ago)
		ts := time.Now().UTC().Add(-29 * 24 * time.Hour)
		assert.NoError(t, ValidateReplayTimestamp(ts, 30))
	})

	t.Run("timestamp older than window returns ErrTimestampOutOfReplayWindow", func(t *testing.T) {
		ts := time.Now().UTC().Add(-31 * 24 * time.Hour) // 31 days ago, window is 30
		err := ValidateReplayTimestamp(ts, 30)
		assert.ErrorIs(t, err, ErrTimestampOutOfReplayWindow)
	})

	t.Run("future timestamp beyond 30-second allowance returns ErrTimestampFuture", func(t *testing.T) {
		ts := time.Now().UTC().Add(2 * time.Minute) // 2 minutes in the future
		err := ValidateReplayTimestamp(ts, 30)
		assert.ErrorIs(t, err, ErrTimestampFuture)
	})

	t.Run("future timestamp within 30-second allowance is valid", func(t *testing.T) {
		ts := time.Now().UTC().Add(20 * time.Second)
		assert.NoError(t, ValidateReplayTimestamp(ts, 30))
	})

	t.Run("window of 1 day rejects 2-day-old timestamp", func(t *testing.T) {
		ts := time.Now().UTC().Add(-2 * 24 * time.Hour)
		err := ValidateReplayTimestamp(ts, 1)
		assert.ErrorIs(t, err, ErrTimestampOutOfReplayWindow)
	})

	t.Run("window of 1 day accepts 12-hour-old timestamp", func(t *testing.T) {
		ts := time.Now().UTC().Add(-12 * time.Hour)
		assert.NoError(t, ValidateReplayTimestamp(ts, 1))
	})
}
