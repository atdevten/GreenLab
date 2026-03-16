package application

import (
	"context"
	"errors"
	"log/slog"
	"testing"
	"time"

	"github.com/google/uuid"
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

// --- helpers ---

// newTestIngestService creates a service with maxReadingAge=0 (timestamp validation disabled)
// so tests can use arbitrary timestamps without hitting the age window check.
func newTestIngestService(t *testing.T) (*IngestService, *mockPublisher) {
	t.Helper()
	p := &mockPublisher{}
	svc := NewIngestService(p, slog.Default(), 0)
	return svc, p
}

// --- tests ---

func TestIngest(t *testing.T) {
	ctx := context.Background()

	t.Run("success", func(t *testing.T) {
		svc, p := newTestIngestService(t)
		channelID := uuid.New().String()

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
			ChannelID: uuid.New().String(),
			Fields:    map[string]float64{},
		})
		assert.ErrorIs(t, err, domain.ErrEmptyFields)
	})

	t.Run("future timestamp returns domain error", func(t *testing.T) {
		p := &mockPublisher{}
		svc := NewIngestService(p, slog.Default(), 24*time.Hour)
		future := time.Now().UTC().Add(time.Hour)
		err := svc.Ingest(ctx, IngestInput{
			ChannelID: uuid.New().String(),
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
			ChannelID: uuid.New().String(),
			Fields:    map[string]float64{"x": 1},
			Timestamp: &old,
		})
		assert.ErrorIs(t, err, domain.ErrTimestampTooOld)
	})

	t.Run("custom timestamp is preserved", func(t *testing.T) {
		svc, p := newTestIngestService(t)
		channelID := uuid.New().String()
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
		channelID := uuid.New().String()
		kafkaErr := errors.New("kafka down")

		p.On("PublishReadings", ctx, mock.AnythingOfType("[]*domain.Reading")).Return(kafkaErr)

		err := svc.Ingest(ctx, IngestInput{ChannelID: channelID, Fields: map[string]float64{"x": 1}})
		assert.ErrorIs(t, err, kafkaErr)
		p.AssertExpectations(t)
	})
}

func TestIngestBatch(t *testing.T) {
	ctx := context.Background()

	t.Run("success", func(t *testing.T) {
		svc, p := newTestIngestService(t)

		inputs := []IngestInput{
			{ChannelID: uuid.New().String(), Fields: map[string]float64{"a": 1}},
			{ChannelID: uuid.New().String(), Fields: map[string]float64{"b": 2}},
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
			{ChannelID: uuid.New().String(), Fields: map[string]float64{"a": 1}},
			{ChannelID: "bad-uuid", Fields: map[string]float64{"b": 2}},
		})
		assert.ErrorIs(t, err, domain.ErrInvalidChannelID)
		assert.ErrorContains(t, err, "item 1")
	})

	t.Run("empty fields in batch returns domain error with index", func(t *testing.T) {
		svc, _ := newTestIngestService(t)
		err := svc.IngestBatch(ctx, []IngestInput{
			{ChannelID: uuid.New().String(), Fields: map[string]float64{"a": 1}},
			{ChannelID: uuid.New().String(), Fields: map[string]float64{}},
		})
		assert.ErrorIs(t, err, domain.ErrEmptyFields)
		assert.ErrorContains(t, err, "item 1")
	})

	t.Run("publisher error is returned", func(t *testing.T) {
		svc, p := newTestIngestService(t)
		kafkaErr := errors.New("kafka batch error")

		p.On("PublishReadings", ctx, mock.AnythingOfType("[]*domain.Reading")).Return(kafkaErr)

		err := svc.IngestBatch(ctx, []IngestInput{
			{ChannelID: uuid.New().String(), Fields: map[string]float64{"x": 1}},
		})
		assert.ErrorIs(t, err, kafkaErr)
		p.AssertExpectations(t)
	})
}
