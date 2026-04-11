package application

import (
	"context"
	"errors"
	"log/slog"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/greenlab/ingestion/internal/domain"
)

// --- mock EventPublisher ---

type mockPublisher struct{ mock.Mock }

func (m *mockPublisher) PublishReadings(ctx context.Context, readings []*domain.Reading) error {
	return m.Called(ctx, readings).Error(0)
}

func (m *mockPublisher) PublishReplayReadings(ctx context.Context, readings []*domain.Reading) error {
	return m.Called(ctx, readings).Error(0)
}

// --- helpers ---

// newTestIngestService creates a service with maxReadingAge=0 (timestamp validation disabled)
// so tests can use arbitrary timestamps without hitting the age window check.
func newTestIngestService(t *testing.T) (*IngestService, *mockPublisher) {
	t.Helper()
	p := &mockPublisher{}
	svc := NewIngestService(p, slog.Default(), 0)
	return svc, p
}

const (
	testChannelID  = "550e8400-e29b-41d4-a716-446655440001"
	testChannelID2 = "550e8400-e29b-41d4-a716-446655440002"
)

// --- tests ---

func TestIngest(t *testing.T) {
	ctx := context.Background()

	t.Run("success", func(t *testing.T) {
		svc, p := newTestIngestService(t)
		channelID := testChannelID

		p.On("PublishReadings", ctx, mock.AnythingOfType("[]*domain.Reading")).Return(nil)

		err := svc.Ingest(ctx, IngestInput{
			ChannelID: channelID,
			DeviceID:  "dev-1",
			Fields:    map[string]float64{"temp": 22.5},
		})
		require.NoError(t, err)
		p.AssertExpectations(t)
	})

	t.Run("invalid channel_id returns domain error", func(t *testing.T) {
		svc, _ := newTestIngestService(t)
		err := svc.Ingest(ctx, IngestInput{
			ChannelID: "not-a-uuid",
			Fields:    map[string]float64{"temp": 22.5},
		})
		assert.ErrorIs(t, err, domain.ErrInvalidChannelID)
	})

	t.Run("empty fields returns domain error", func(t *testing.T) {
		svc, _ := newTestIngestService(t)
		err := svc.Ingest(ctx, IngestInput{
			ChannelID: testChannelID,
			Fields:    map[string]float64{},
		})
		assert.ErrorIs(t, err, domain.ErrEmptyFields)
	})

	t.Run("future timestamp returns domain error", func(t *testing.T) {
		p := &mockPublisher{}
		svc := NewIngestService(p, slog.Default(), 24*time.Hour)
		future := time.Now().UTC().Add(time.Hour)
		err := svc.Ingest(ctx, IngestInput{
			ChannelID: testChannelID,
			Fields:    map[string]float64{"x": 1},
			Timestamp: &future,
		})
		assert.ErrorIs(t, err, domain.ErrTimestampFuture)
	})

	t.Run("too-old timestamp returns domain error", func(t *testing.T) {
		p := &mockPublisher{}
		svc := NewIngestService(p, slog.Default(), 24*time.Hour)
		old := time.Now().UTC().Add(-48 * time.Hour)
		err := svc.Ingest(ctx, IngestInput{
			ChannelID: testChannelID,
			Fields:    map[string]float64{"x": 1},
			Timestamp: &old,
		})
		assert.ErrorIs(t, err, domain.ErrTimestampTooOld)
	})

	t.Run("custom timestamp is preserved", func(t *testing.T) {
		svc, p := newTestIngestService(t)
		channelID := testChannelID
		ts := time.Now().UTC().Add(-time.Minute) // recent, valid timestamp

		p.On("PublishReadings", ctx, mock.MatchedBy(func(readings []*domain.Reading) bool {
			return len(readings) == 1 && readings[0].Timestamp.Equal(ts)
		})).Return(nil)

		err := svc.Ingest(ctx, IngestInput{
			ChannelID: channelID,
			Fields:    map[string]float64{"temp": 1.0},
			Timestamp: &ts,
		})
		require.NoError(t, err)
		p.AssertExpectations(t)
	})

	t.Run("publisher error is returned", func(t *testing.T) {
		svc, p := newTestIngestService(t)
		channelID := testChannelID
		kafkaErr := errors.New("kafka down")

		p.On("PublishReadings", ctx, mock.AnythingOfType("[]*domain.Reading")).Return(kafkaErr)

		err := svc.Ingest(ctx, IngestInput{ChannelID: channelID, Fields: map[string]float64{"x": 1}})
		assert.ErrorIs(t, err, kafkaErr)
		p.AssertExpectations(t)
	})

	t.Run("no field timestamps produces one reading", func(t *testing.T) {
		svc, p := newTestIngestService(t)
		channelID := testChannelID

		p.On("PublishReadings", ctx, mock.MatchedBy(func(readings []*domain.Reading) bool {
			return len(readings) == 1
		})).Return(nil)

		err := svc.Ingest(ctx, IngestInput{
			ChannelID: channelID,
			Fields:    map[string]float64{"temp": 22.5, "humidity": 60.0},
		})
		require.NoError(t, err)
		p.AssertExpectations(t)
	})

	t.Run("all fields share same per-field timestamp produces one reading", func(t *testing.T) {
		svc, p := newTestIngestService(t)
		channelID := testChannelID
		sharedTS := time.Now().UTC().Add(-time.Minute)

		p.On("PublishReadings", ctx, mock.MatchedBy(func(readings []*domain.Reading) bool {
			if len(readings) != 1 {
				return false
			}
			return readings[0].Timestamp.Equal(sharedTS) &&
				len(readings[0].Fields) == 2
		})).Return(nil)

		err := svc.Ingest(ctx, IngestInput{
			ChannelID: channelID,
			Fields:    map[string]float64{"temp": 22.5, "humidity": 60.0},
			FieldTimestamps: map[string]*time.Time{
				"temp":     &sharedTS,
				"humidity": &sharedTS,
			},
		})
		require.NoError(t, err)
		p.AssertExpectations(t)
	})

	t.Run("fields with different timestamps produce multiple readings", func(t *testing.T) {
		svc, p := newTestIngestService(t)
		channelID := testChannelID
		ts1 := time.Now().UTC().Add(-2 * time.Minute)
		ts2 := time.Now().UTC().Add(-1 * time.Minute)

		p.On("PublishReadings", ctx, mock.MatchedBy(func(readings []*domain.Reading) bool {
			return len(readings) == 2
		})).Return(nil)

		err := svc.Ingest(ctx, IngestInput{
			ChannelID: channelID,
			Fields:    map[string]float64{"temp": 22.5, "humidity": 60.0},
			FieldTimestamps: map[string]*time.Time{
				"temp":     &ts1,
				"humidity": &ts2,
			},
		})
		require.NoError(t, err)
		p.AssertExpectations(t)
	})

	t.Run("per-field timestamp too old returns error", func(t *testing.T) {
		p := &mockPublisher{}
		svc := NewIngestService(p, slog.Default(), 24*time.Hour)
		channelID := testChannelID
		oldTS := time.Now().UTC().Add(-48 * time.Hour)

		err := svc.Ingest(ctx, IngestInput{
			ChannelID: channelID,
			Fields:    map[string]float64{"temp": 22.5},
			FieldTimestamps: map[string]*time.Time{
				"temp": &oldTS,
			},
		})
		assert.ErrorIs(t, err, domain.ErrTimestampTooOld)
		p.AssertNotCalled(t, "PublishReadings")
	})

	t.Run("per-field timestamp in the future returns error", func(t *testing.T) {
		p := &mockPublisher{}
		svc := NewIngestService(p, slog.Default(), 24*time.Hour)
		channelID := testChannelID
		futureTS := time.Now().UTC().Add(time.Hour)

		err := svc.Ingest(ctx, IngestInput{
			ChannelID: channelID,
			Fields:    map[string]float64{"temp": 22.5},
			FieldTimestamps: map[string]*time.Time{
				"temp": &futureTS,
			},
		})
		assert.ErrorIs(t, err, domain.ErrTimestampFuture)
		p.AssertNotCalled(t, "PublishReadings")
	})

	t.Run("mixed: some fields have timestamps, others fall back to default", func(t *testing.T) {
		svc, p := newTestIngestService(t)
		channelID := testChannelID
		defaultTS := time.Now().UTC().Add(-5 * time.Minute)
		fieldTS := time.Now().UTC().Add(-1 * time.Minute)

		// temp uses fieldTS, humidity falls back to defaultTS — two distinct timestamps → two readings
		p.On("PublishReadings", ctx, mock.MatchedBy(func(readings []*domain.Reading) bool {
			return len(readings) == 2
		})).Return(nil)

		err := svc.Ingest(ctx, IngestInput{
			ChannelID: channelID,
			Fields:    map[string]float64{"temp": 22.5, "humidity": 60.0},
			FieldTimestamps: map[string]*time.Time{
				"temp": &fieldTS,
				// humidity has no entry → falls back to Timestamp/defaultTS
			},
			Timestamp: &defaultTS,
		})
		require.NoError(t, err)
		p.AssertExpectations(t)
	})

	t.Run("nil per-field timestamp entry falls back to default", func(t *testing.T) {
		svc, p := newTestIngestService(t)
		channelID := testChannelID

		// Both fields end up at defaultTS (nil entry treated as absent) → one reading
		p.On("PublishReadings", ctx, mock.MatchedBy(func(readings []*domain.Reading) bool {
			return len(readings) == 1 && len(readings[0].Fields) == 2
		})).Return(nil)

		var nilTS *time.Time
		err := svc.Ingest(ctx, IngestInput{
			ChannelID: channelID,
			Fields:    map[string]float64{"temp": 22.5, "humidity": 60.0},
			FieldTimestamps: map[string]*time.Time{
				"temp": nilTS,
			},
		})
		require.NoError(t, err)
		p.AssertExpectations(t)
	})
}

func TestIngestBatch(t *testing.T) {
	ctx := context.Background()

	t.Run("success", func(t *testing.T) {
		svc, p := newTestIngestService(t)

		inputs := []IngestInput{
			{ChannelID: testChannelID, Fields: map[string]float64{"a": 1}},
			{ChannelID: testChannelID2, Fields: map[string]float64{"b": 2}},
		}

		p.On("PublishReadings", ctx, mock.AnythingOfType("[]*domain.Reading")).Return(nil)

		err := svc.IngestBatch(ctx, inputs)
		require.NoError(t, err)
		p.AssertExpectations(t)
	})

	t.Run("empty batch is a no-op", func(t *testing.T) {
		svc, p := newTestIngestService(t)
		err := svc.IngestBatch(ctx, []IngestInput{})
		require.NoError(t, err)
		p.AssertNotCalled(t, "PublishReadings")
	})

	t.Run("invalid channel_id in batch returns domain error with index", func(t *testing.T) {
		svc, _ := newTestIngestService(t)
		err := svc.IngestBatch(ctx, []IngestInput{
			{ChannelID: testChannelID, Fields: map[string]float64{"a": 1}},
			{ChannelID: "bad-uuid", Fields: map[string]float64{"b": 2}},
		})
		assert.ErrorIs(t, err, domain.ErrInvalidChannelID)
		assert.ErrorContains(t, err, "item 1")
	})

	t.Run("empty fields in batch returns domain error with index", func(t *testing.T) {
		svc, _ := newTestIngestService(t)
		err := svc.IngestBatch(ctx, []IngestInput{
			{ChannelID: testChannelID, Fields: map[string]float64{"a": 1}},
			{ChannelID: testChannelID, Fields: map[string]float64{}},
		})
		assert.ErrorIs(t, err, domain.ErrEmptyFields)
		assert.ErrorContains(t, err, "item 1")
	})

	t.Run("publisher error is returned", func(t *testing.T) {
		svc, p := newTestIngestService(t)
		kafkaErr := errors.New("kafka batch error")

		p.On("PublishReadings", ctx, mock.AnythingOfType("[]*domain.Reading")).Return(kafkaErr)

		err := svc.IngestBatch(ctx, []IngestInput{
			{ChannelID: testChannelID, Fields: map[string]float64{"x": 1}},
		})
		assert.ErrorIs(t, err, kafkaErr)
		p.AssertExpectations(t)
	})

	t.Run("batch item with per-field timestamps expands into multiple readings", func(t *testing.T) {
		svc, p := newTestIngestService(t)
		channelID := testChannelID
		ts1 := time.Now().UTC().Add(-3 * time.Minute)
		ts2 := time.Now().UTC().Add(-1 * time.Minute)

		// One batch item with 2 distinct field timestamps → 2 readings published
		p.On("PublishReadings", ctx, mock.MatchedBy(func(readings []*domain.Reading) bool {
			return len(readings) == 2
		})).Return(nil)

		err := svc.IngestBatch(ctx, []IngestInput{
			{
				ChannelID: channelID,
				Fields:    map[string]float64{"temp": 20.0, "pressure": 1013.0},
				FieldTimestamps: map[string]*time.Time{
					"temp":     &ts1,
					"pressure": &ts2,
				},
			},
		})
		require.NoError(t, err)
		p.AssertExpectations(t)
	})

	t.Run("batch item per-field timestamp invalid returns error with item index", func(t *testing.T) {
		p := &mockPublisher{}
		svc := NewIngestService(p, slog.Default(), 24*time.Hour)
		futureTS := time.Now().UTC().Add(time.Hour)

		err := svc.IngestBatch(ctx, []IngestInput{
			{ChannelID: testChannelID, Fields: map[string]float64{"a": 1}},
			{
				ChannelID: testChannelID,
				Fields:    map[string]float64{"temp": 20.0},
				FieldTimestamps: map[string]*time.Time{
					"temp": &futureTS,
				},
			},
		})
		assert.ErrorIs(t, err, domain.ErrTimestampFuture)
		assert.ErrorContains(t, err, "item 1")
		p.AssertNotCalled(t, "PublishReadings")
	})
}

// --- mockReplayDLQ ---

type mockReplayDLQ struct{ mock.Mock }

func (m *mockReplayDLQ) Push(ctx context.Context, entry ReplayDLQEntry) error {
	return m.Called(ctx, entry).Error(0)
}

func (m *mockReplayDLQ) IncrFailureMetric(ctx context.Context) error {
	return m.Called(ctx).Error(0)
}

// --- IngestReplay tests ---

func TestIngestReplay(t *testing.T) {
	ctx := context.Background()
	channelID := testChannelID

	t.Run("success — publishes with replay header and returns nil", func(t *testing.T) {
		svc, p := newTestIngestService(t)
		dlq := &mockReplayDLQ{}

		ts := time.Now().UTC().Add(-24 * time.Hour)
		p.On("PublishReplayReadings", ctx, mock.AnythingOfType("[]*domain.Reading")).Return(nil)

		err := svc.IngestReplay(ctx, []IngestInput{
			{ChannelID: channelID, DeviceID: "dev-1", Fields: map[string]float64{"temp": 22.5}, Timestamp: &ts},
		}, 30, dlq)
		require.NoError(t, err)
		p.AssertExpectations(t)
		dlq.AssertNotCalled(t, "Push")
	})

	t.Run("empty inputs returns nil without publishing", func(t *testing.T) {
		svc, p := newTestIngestService(t)
		dlq := &mockReplayDLQ{}

		err := svc.IngestReplay(ctx, nil, 30, dlq)
		require.NoError(t, err)
		p.AssertNotCalled(t, "PublishReplayReadings")
		dlq.AssertNotCalled(t, "Push")
	})

	t.Run("invalid channel_id returns validation error", func(t *testing.T) {
		svc, p := newTestIngestService(t)
		dlq := &mockReplayDLQ{}

		err := svc.IngestReplay(ctx, []IngestInput{
			{ChannelID: "not-a-uuid", Fields: map[string]float64{"temp": 1}},
		}, 30, dlq)
		assert.ErrorIs(t, err, domain.ErrInvalidChannelID)
		p.AssertNotCalled(t, "PublishReplayReadings")
	})

	t.Run("empty fields returns validation error", func(t *testing.T) {
		svc, p := newTestIngestService(t)
		dlq := &mockReplayDLQ{}

		err := svc.IngestReplay(ctx, []IngestInput{
			{ChannelID: channelID, Fields: nil},
		}, 30, dlq)
		assert.ErrorIs(t, err, domain.ErrEmptyFields)
		p.AssertNotCalled(t, "PublishReplayReadings")
	})

	t.Run("timestamp outside replay window returns ErrTimestampOutOfReplayWindow", func(t *testing.T) {
		svc, p := newTestIngestService(t)
		dlq := &mockReplayDLQ{}

		oldTS := time.Now().UTC().Add(-60 * 24 * time.Hour) // 60 days ago, window is 30
		err := svc.IngestReplay(ctx, []IngestInput{
			{ChannelID: channelID, Fields: map[string]float64{"temp": 1}, Timestamp: &oldTS},
		}, 30, dlq)
		assert.ErrorIs(t, err, domain.ErrTimestampOutOfReplayWindow)
		p.AssertNotCalled(t, "PublishReplayReadings")
	})

	t.Run("future timestamp returns ErrTimestampFuture", func(t *testing.T) {
		svc, p := newTestIngestService(t)
		dlq := &mockReplayDLQ{}

		futureTS := time.Now().UTC().Add(2 * time.Minute)
		err := svc.IngestReplay(ctx, []IngestInput{
			{ChannelID: channelID, Fields: map[string]float64{"temp": 1}, Timestamp: &futureTS},
		}, 30, dlq)
		assert.ErrorIs(t, err, domain.ErrTimestampFuture)
		p.AssertNotCalled(t, "PublishReplayReadings")
	})

	t.Run("publish failure triggers retries and writes to DLQ", func(t *testing.T) {
		svc, p := newTestIngestService(t)
		dlq := &mockReplayDLQ{}
		publishErr := errors.New("kafka unavailable")

		// All 3 attempts fail.
		p.On("PublishReplayReadings", ctx, mock.AnythingOfType("[]*domain.Reading")).Return(publishErr).Times(3)
		dlq.On("Push", ctx, mock.AnythingOfType("ReplayDLQEntry")).Return(nil)
		dlq.On("IncrFailureMetric", ctx).Return(nil)

		ts := time.Now().UTC().Add(-1 * time.Hour)
		err := svc.IngestReplay(ctx, []IngestInput{
			{ChannelID: channelID, DeviceID: "dev-1", Fields: map[string]float64{"temp": 1}, Timestamp: &ts},
		}, 30, dlq)
		require.Error(t, err)
		assert.ErrorIs(t, err, publishErr)
		p.AssertExpectations(t)
		dlq.AssertExpectations(t)
	})

	t.Run("publish fails on first attempt then succeeds — no DLQ write", func(t *testing.T) {
		svc, p := newTestIngestService(t)
		dlq := &mockReplayDLQ{}
		publishErr := errors.New("transient kafka error")

		// First call fails, second call succeeds.
		p.On("PublishReplayReadings", ctx, mock.AnythingOfType("[]*domain.Reading")).
			Return(publishErr).Once()
		p.On("PublishReplayReadings", ctx, mock.AnythingOfType("[]*domain.Reading")).
			Return(nil).Once()

		ts := time.Now().UTC().Add(-1 * time.Hour)
		err := svc.IngestReplay(ctx, []IngestInput{
			{ChannelID: channelID, DeviceID: "dev-1", Fields: map[string]float64{"temp": 1}, Timestamp: &ts},
		}, 30, dlq)
		require.NoError(t, err)
		p.AssertExpectations(t)
		dlq.AssertNotCalled(t, "Push")
	})

	t.Run("item at index 1 fails validation — error includes item index", func(t *testing.T) {
		svc, p := newTestIngestService(t)
		dlq := &mockReplayDLQ{}

		ts := time.Now().UTC().Add(-1 * time.Hour)
		err := svc.IngestReplay(ctx, []IngestInput{
			{ChannelID: channelID, DeviceID: "dev-1", Fields: map[string]float64{"temp": 1}, Timestamp: &ts},
			{ChannelID: "not-a-uuid", Fields: map[string]float64{"temp": 2}},
		}, 30, dlq)
		assert.ErrorIs(t, err, domain.ErrInvalidChannelID)
		assert.ErrorContains(t, err, "item 1")
		p.AssertNotCalled(t, "PublishReplayReadings")
	})
}
