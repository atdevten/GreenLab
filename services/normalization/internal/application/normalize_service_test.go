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
	"github.com/greenlab/normalization/internal/domain"
)

// --- inline mocks ---

type mockInfluxWriter struct{ mock.Mock }

func (m *mockInfluxWriter) Write(ctx context.Context, r *domain.ReadingPayload) error {
	return m.Called(ctx, r).Error(0)
}

type mockEventPublisher struct{ mock.Mock }

func (m *mockEventPublisher) PublishReading(ctx context.Context, evt *domain.ReadingEvent) error {
	return m.Called(ctx, evt).Error(0)
}

// --- helpers ---

func newTestService(t *testing.T) (*NormalizationService, *mockInfluxWriter, *mockEventPublisher) {
	t.Helper()
	w := &mockInfluxWriter{}
	p := &mockEventPublisher{}
	svc := NewNormalizationService(w, p, slog.Default())
	return svc, w, p
}

func sampleEvent() *domain.ReadingEvent {
	return &domain.ReadingEvent{
		ID:          "test-id-001",
		Type:        "reading.ingested",
		PublishedAt: time.Now().UTC(),
		Reading: domain.ReadingPayload{
			ChannelID: "channel-abc",
			DeviceID:  "device-xyz",
			Fields:    map[string]float64{"temperature": 23.5, "humidity": 60.0},
			Tags:      map[string]string{"location": "lab"},
			Timestamp: time.Now().UTC(),
		},
	}
}

// --- tests ---

func TestNormalizationService_Process(t *testing.T) {
	ctx := context.Background()

	t.Run("success", func(t *testing.T) {
		svc, w, p := newTestService(t)
		evt := sampleEvent()

		w.On("Write", ctx, &evt.Reading).Return(nil)
		p.On("PublishReading", ctx, evt).Return(nil)

		err := svc.Process(ctx, evt)
		require.NoError(t, err)
		w.AssertExpectations(t)
		p.AssertExpectations(t)
	})

	t.Run("empty fields returns error", func(t *testing.T) {
		svc, w, _ := newTestService(t)
		evt := sampleEvent()
		evt.Reading.Fields = map[string]float64{}

		err := svc.Process(ctx, evt)
		assert.ErrorContains(t, err, "no fields")
		w.AssertNotCalled(t, "Write")
	})

	t.Run("influxdb write error is returned", func(t *testing.T) {
		svc, w, p := newTestService(t)
		evt := sampleEvent()
		writeErr := errors.New("influx unavailable")

		w.On("Write", ctx, &evt.Reading).Return(writeErr)

		err := svc.Process(ctx, evt)
		assert.ErrorIs(t, err, writeErr)
		w.AssertExpectations(t)
		p.AssertNotCalled(t, "PublishReading")
	})

	t.Run("publisher error is logged but not returned", func(t *testing.T) {
		svc, w, p := newTestService(t)
		evt := sampleEvent()

		w.On("Write", ctx, &evt.Reading).Return(nil)
		p.On("PublishReading", ctx, evt).Return(errors.New("kafka down"))

		err := svc.Process(ctx, evt)
		require.NoError(t, err)
		w.AssertExpectations(t)
		p.AssertExpectations(t)
	})
}
