package redis

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/go-redis/redismock/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/greenlab/ingestion/internal/application"
)

func makeEntry(channelID string) application.ReplayDLQEntry {
	return application.ReplayDLQEntry{
		ChannelID: channelID,
		DeviceID:  "dev-1",
		Timestamp: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		Fields:    map[string]float64{"temp": 22.5},
		FailedAt:  time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC),
	}
}

func TestReplayDLQ_Push_Success(t *testing.T) {
	client, mock := redismock.NewClientMock()
	dlq := NewReplayDLQ(client)
	ctx := context.Background()

	entry := makeEntry("chan-abc")
	b, err := json.Marshal(entry)
	require.NoError(t, err)

	mock.ExpectRPush(dlqKey("chan-abc"), b).SetVal(1)

	err = dlq.Push(ctx, entry)
	assert.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestReplayDLQ_Push_RedisError(t *testing.T) {
	client, mock := redismock.NewClientMock()
	dlq := NewReplayDLQ(client)
	ctx := context.Background()

	entry := makeEntry("chan-err")
	b, err := json.Marshal(entry)
	require.NoError(t, err)

	redisErr := errors.New("connection refused")
	mock.ExpectRPush(dlqKey("chan-err"), b).SetErr(redisErr)

	err = dlq.Push(ctx, entry)
	require.Error(t, err)
	assert.ErrorContains(t, err, "ReplayDLQ.Push")
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestReplayDLQ_IncrFailureMetric_Success(t *testing.T) {
	client, mock := redismock.NewClientMock()
	dlq := NewReplayDLQ(client)
	ctx := context.Background()

	mock.ExpectIncr(replayFailureMetricKey).SetVal(1)

	err := dlq.IncrFailureMetric(ctx)
	assert.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestReplayDLQ_IncrFailureMetric_RedisError(t *testing.T) {
	client, mock := redismock.NewClientMock()
	dlq := NewReplayDLQ(client)
	ctx := context.Background()

	redisErr := errors.New("connection refused")
	mock.ExpectIncr(replayFailureMetricKey).SetErr(redisErr)

	err := dlq.IncrFailureMetric(ctx)
	require.Error(t, err)
	assert.ErrorContains(t, err, "ReplayDLQ.IncrFailureMetric")
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestReplayDLQ_dlqKey(t *testing.T) {
	assert.Equal(t, "replay_dlq:chan-123", dlqKey("chan-123"))
}
