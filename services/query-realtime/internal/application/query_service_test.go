package application

import (
	"context"
	"errors"
	"log/slog"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/greenlab/query-realtime/internal/domain/query"
)

// stubCache is a no-op cache that always reports a miss.
type stubCache struct{}

func (s *stubCache) Get(_ context.Context, _ string) ([]byte, error) {
	return nil, errors.New("miss")
}

func (s *stubCache) Set(_ context.Context, _ string, _ []byte, _ time.Duration) error {
	return nil
}

// stubReader returns a canned LatestReading or error.
type stubReader struct {
	latest *query.LatestReading
	err    error
}

func (r *stubReader) Query(_ context.Context, _ *query.QueryRequest) (*query.QueryResult, error) {
	return nil, nil
}

func (r *stubReader) QueryLatest(_ context.Context, _, _ string) (*query.LatestReading, error) {
	return r.latest, r.err
}

func newTestQueryService(reader Reader) *QueryService {
	return NewQueryService(reader, &stubCache{}, slog.Default(), time.Minute, time.Minute)
}

func TestQueryLatest_RejectsInvalidChannelID(t *testing.T) {
	svc := newTestQueryService(&stubReader{})
	_, err := svc.QueryLatest(context.Background(), "not-a-uuid", "temperature")
	assert.ErrorIs(t, err, query.ErrInvalidChannelID)
}

func TestQueryLatest_RejectsEmptyChannelID(t *testing.T) {
	svc := newTestQueryService(&stubReader{})
	_, err := svc.QueryLatest(context.Background(), "", "temperature")
	assert.ErrorIs(t, err, query.ErrInvalidChannelID)
}

func TestQueryLatest_RejectsEmptyFieldName(t *testing.T) {
	svc := newTestQueryService(&stubReader{})
	_, err := svc.QueryLatest(context.Background(), uuid.New().String(), "")
	assert.ErrorIs(t, err, query.ErrInvalidFieldName)
}

func TestQueryLatest_RejectsInvalidFieldName(t *testing.T) {
	svc := newTestQueryService(&stubReader{})
	_, err := svc.QueryLatest(context.Background(), uuid.New().String(), "bad field!")
	assert.ErrorIs(t, err, query.ErrInvalidFieldName)
}

func TestQueryLatest_ValidInputCallsReader(t *testing.T) {
	chID := uuid.New().String()
	expected := &query.LatestReading{
		ChannelID: chID,
		FieldName: "temperature",
		Value:     42.0,
		Timestamp: time.Now().UTC().Truncate(time.Second),
	}
	svc := newTestQueryService(&stubReader{latest: expected})
	result, err := svc.QueryLatest(context.Background(), chID, "temperature")
	require.NoError(t, err)
	assert.Equal(t, expected, result)
}

func TestQueryLatest_ReaderErrorPropagated(t *testing.T) {
	readerErr := errors.New("influxdb unavailable")
	svc := newTestQueryService(&stubReader{err: readerErr})
	_, err := svc.QueryLatest(context.Background(), uuid.New().String(), "temperature")
	assert.ErrorIs(t, err, readerErr)
}
