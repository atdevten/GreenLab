package application

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/greenlab/ingestion/internal/domain"
)

// EventPublisher publishes ingestion events.
type EventPublisher interface {
	PublishReadings(ctx context.Context, readings []*domain.Reading) error
}

// IngestService handles write-side telemetry ingestion.
type IngestService struct {
	publisher     EventPublisher
	logger        *slog.Logger
	maxReadingAge time.Duration
}

// NewIngestService creates an IngestService. maxReadingAge bounds how far in the
// past a client-supplied timestamp may be; pass 0 to disable the past-bound check.
func NewIngestService(publisher EventPublisher, logger *slog.Logger, maxReadingAge time.Duration) *IngestService {
	return &IngestService{
		publisher:     publisher,
		logger:        logger,
		maxReadingAge: maxReadingAge,
	}
}

// IngestInput represents a single ingestion request.
type IngestInput struct {
	ChannelID string
	DeviceID  string
	Fields    map[string]float64
	Tags      map[string]string
	Timestamp *time.Time
}

// Ingest validates a single reading and publishes it to Kafka.
func (s *IngestService) Ingest(ctx context.Context, in IngestInput) error {
	if err := domain.ValidateChannelID(in.ChannelID); err != nil {
		return fmt.Errorf("Ingest: %w", err)
	}
	if len(in.Fields) == 0 {
		return fmt.Errorf("Ingest: %w", domain.ErrEmptyFields)
	}
	ts := time.Now().UTC()
	if in.Timestamp != nil {
		if err := domain.ValidateTimestamp(*in.Timestamp, s.maxReadingAge); err != nil {
			return fmt.Errorf("Ingest: %w", err)
		}
		ts = *in.Timestamp
	}
	reading := domain.NewReading(in.ChannelID, in.DeviceID, in.Fields, in.Tags, ts)
	if err := s.publisher.PublishReadings(ctx, []*domain.Reading{reading}); err != nil {
		return fmt.Errorf("Ingest.Publish: %w", err)
	}
	return nil
}

// IngestBatch validates and publishes multiple readings in a batch.
func (s *IngestService) IngestBatch(ctx context.Context, readings []IngestInput) error {
	if len(readings) == 0 {
		return nil
	}
	now := time.Now().UTC()
	batch := make([]*domain.Reading, 0, len(readings))
	for i, in := range readings {
		if err := domain.ValidateChannelID(in.ChannelID); err != nil {
			return fmt.Errorf("IngestBatch: item %d: %w", i, err)
		}
		if len(in.Fields) == 0 {
			return fmt.Errorf("IngestBatch: item %d: %w", i, domain.ErrEmptyFields)
		}
		ts := now
		if in.Timestamp != nil {
			if err := domain.ValidateTimestamp(*in.Timestamp, s.maxReadingAge); err != nil {
				return fmt.Errorf("IngestBatch: item %d: %w", i, err)
			}
			ts = *in.Timestamp
		}
		batch = append(batch, domain.NewReading(in.ChannelID, in.DeviceID, in.Fields, in.Tags, ts))
	}
	if err := s.publisher.PublishReadings(ctx, batch); err != nil {
		return fmt.Errorf("IngestBatch.Publish: %w", err)
	}
	return nil
}
