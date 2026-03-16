package application

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/greenlab/normalization/internal/domain"
)

// InfluxWriter writes readings to InfluxDB.
type InfluxWriter interface {
	Write(ctx context.Context, r *domain.ReadingPayload) error
}

// EventPublisher publishes normalized readings downstream.
type EventPublisher interface {
	PublishReading(ctx context.Context, evt *domain.ReadingEvent) error
}

// NormalizationService processes raw reading events from the ingest pipeline.
type NormalizationService struct {
	writer    InfluxWriter
	publisher EventPublisher
	logger    *slog.Logger
}

// NewNormalizationService creates a NormalizationService.
func NewNormalizationService(writer InfluxWriter, publisher EventPublisher, logger *slog.Logger) *NormalizationService {
	return &NormalizationService{
		writer:    writer,
		publisher: publisher,
		logger:    logger,
	}
}

// Process validates the event, writes the reading to InfluxDB, then publishes to
// the normalized topic. InfluxDB write failure is returned as a hard error — data
// would be irretrievably lost. Publisher failure is logged but not fatal; the raw
// event remains on raw.sensor.ingest and can be replayed.
func (s *NormalizationService) Process(ctx context.Context, evt *domain.ReadingEvent) error {
	if len(evt.Reading.Fields) == 0 {
		return fmt.Errorf("Process: reading has no fields")
	}

	if err := s.writer.Write(ctx, &evt.Reading); err != nil {
		return fmt.Errorf("Process.Write: %w", err)
	}

	if err := s.publisher.PublishReading(ctx, evt); err != nil {
		s.logger.Error("failed to publish normalized reading event",
			"event_id", evt.ID,
			"channel_id", evt.Reading.ChannelID,
			"error", err,
		)
	}
	return nil
}
