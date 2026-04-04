package domain

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

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
