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
	ChannelID       string
	DeviceID        string
	Fields          map[string]float64
	FieldTimestamps map[string]*time.Time
	Tags            map[string]string
	Timestamp       *time.Time
}

// Ingest validates a single reading and publishes it to Kafka.
// If FieldTimestamps are provided, fields with different effective timestamps are
// grouped into separate Reading objects, each published in one batch call.
func (s *IngestService) Ingest(ctx context.Context, in IngestInput) error {
	if err := domain.ValidateChannelID(in.ChannelID); err != nil {
		return fmt.Errorf("Ingest: %w", err)
	}
	if len(in.Fields) == 0 {
		return fmt.Errorf("Ingest: %w", domain.ErrEmptyFields)
	}

	defaultTS := time.Now().UTC()
	if in.Timestamp != nil {
		if err := domain.ValidateTimestamp(*in.Timestamp, s.maxReadingAge); err != nil {
			return fmt.Errorf("Ingest: %w", err)
		}
		defaultTS = *in.Timestamp
	}

	readings, err := s.groupByTimestamp(in.ChannelID, in.DeviceID, in.Fields, in.FieldTimestamps, in.Tags, defaultTS)
	if err != nil {
		return fmt.Errorf("Ingest: %w", err)
	}

	if err := s.publisher.PublishReadings(ctx, readings); err != nil {
		return fmt.Errorf("Ingest.Publish: %w", err)
	}
	return nil
}

// IngestBatch validates and publishes multiple readings in a batch.
// Per-field timestamps are respected for each item in the batch.
func (s *IngestService) IngestBatch(ctx context.Context, inputs []IngestInput) error {
	if len(inputs) == 0 {
		return nil
	}
	now := time.Now().UTC()
	batch := make([]*domain.Reading, 0, len(inputs))
	for i, in := range inputs {
		if err := domain.ValidateChannelID(in.ChannelID); err != nil {
			return fmt.Errorf("IngestBatch: item %d: %w", i, err)
		}
		if len(in.Fields) == 0 {
			return fmt.Errorf("IngestBatch: item %d: %w", i, domain.ErrEmptyFields)
		}
		defaultTS := now
		if in.Timestamp != nil {
			if err := domain.ValidateTimestamp(*in.Timestamp, s.maxReadingAge); err != nil {
				return fmt.Errorf("IngestBatch: item %d: %w", i, err)
			}
			defaultTS = *in.Timestamp
		}
		grouped, err := s.groupByTimestamp(in.ChannelID, in.DeviceID, in.Fields, in.FieldTimestamps, in.Tags, defaultTS)
		if err != nil {
			return fmt.Errorf("IngestBatch: item %d: %w", i, err)
		}
		batch = append(batch, grouped...)
	}
	if err := s.publisher.PublishReadings(ctx, batch); err != nil {
		return fmt.Errorf("IngestBatch.Publish: %w", err)
	}
	return nil
}

// groupByTimestamp groups fields by their effective timestamp and returns one Reading
// per unique timestamp. Fields with a valid per-field timestamp use that; otherwise
// defaultTS is used. Per-field timestamps are validated against maxReadingAge.
func (s *IngestService) groupByTimestamp(
	channelID, deviceID string,
	fields map[string]float64,
	fieldTimestamps map[string]*time.Time,
	tags map[string]string,
	defaultTS time.Time,
) ([]*domain.Reading, error) {
	// Use a slice of (time.Time, map) pairs so ordering is deterministic.
	// A small linear scan is fine because field counts are always small.
	type group struct {
		ts     time.Time
		fields map[string]float64
	}
	var groups []group

	findOrCreate := func(ts time.Time) map[string]float64 {
		for i := range groups {
			if groups[i].ts.Equal(ts) {
				return groups[i].fields
			}
		}
		m := map[string]float64{}
		groups = append(groups, group{ts: ts, fields: m})
		return m
	}

	for fieldName, value := range fields {
		ts := defaultTS
		if ft, ok := fieldTimestamps[fieldName]; ok && ft != nil {
			if err := domain.ValidateTimestamp(*ft, s.maxReadingAge); err != nil {
				return nil, fmt.Errorf("field %q: %w", fieldName, err)
			}
			ts = *ft
		}
		findOrCreate(ts)[fieldName] = value
	}

	readings := make([]*domain.Reading, len(groups))
	for i, g := range groups {
		readings[i] = domain.NewReading(channelID, deviceID, g.fields, tags, g.ts)
	}
	return readings, nil
}
