package application

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/greenlab/query-realtime/internal/domain/query"
)

// Reader is the interface for reading telemetry data from a time-series store.
type Reader interface {
	Query(ctx context.Context, req *query.QueryRequest) (*query.QueryResult, error)
	QueryLatest(ctx context.Context, channelID, fieldName string) (*query.LatestReading, error)
}

// Cache is the interface for a short-lived result cache.
type Cache interface {
	Get(ctx context.Context, key string) ([]byte, error)
	Set(ctx context.Context, key string, value []byte, ttl time.Duration) error
}

// QueryService implements read-side query use-cases.
type QueryService struct {
	reader         Reader
	cache          Cache
	logger         *slog.Logger
	cacheTTL       time.Duration
	latestCacheTTL time.Duration
}

// NewQueryService creates a new QueryService.
func NewQueryService(reader Reader, cache Cache, logger *slog.Logger, cacheTTL, latestCacheTTL time.Duration) *QueryService {
	return &QueryService{
		reader:         reader,
		cache:          cache,
		logger:         logger,
		cacheTTL:       cacheTTL,
		latestCacheTTL: latestCacheTTL,
	}
}

// Query executes a time-series query with optional caching.
// It accepts query.QueryRequest directly; time defaults and limit clamping are
// applied here so the Reader always receives a fully-normalised request.
func (s *QueryService) Query(ctx context.Context, req query.QueryRequest) (*query.QueryResult, error) {
	// Apply time defaults before validation so ErrInvalidTimeRange is only
	// triggered when the caller explicitly supplies a bad range.
	if req.End.IsZero() {
		req.End = time.Now().UTC()
	}
	if req.Start.IsZero() {
		req.Start = req.End.Add(-24 * time.Hour)
	}
	if !req.Start.Before(req.End) {
		return nil, query.ErrInvalidTimeRange
	}

	// Validate all user-supplied strings before any Flux interpolation.
	if err := query.ValidateQueryRequest(&req); err != nil {
		return nil, err
	}

	if req.Limit <= 0 || req.Limit > 10000 {
		req.Limit = 1000
	}

	cacheKey := fmt.Sprintf("query:%s:%s:%d:%d:%s:%s",
		req.ChannelID, req.FieldName,
		req.Start.Unix(), req.End.Unix(),
		req.Aggregate, req.Window,
	)

	if cached, err := s.cache.Get(ctx, cacheKey); err == nil {
		var result query.QueryResult
		if json.Unmarshal(cached, &result) == nil {
			return &result, nil
		}
	}

	result, err := s.reader.Query(ctx, &req)
	if err != nil {
		return nil, err
	}

	if b, err := json.Marshal(result); err == nil {
		if err := s.cache.Set(ctx, cacheKey, b, s.cacheTTL); err != nil {
			s.logger.Warn("failed to cache query result", "error", err)
		}
	}

	return result, nil
}

// QueryLatest returns the most recent reading for a channel field.
func (s *QueryService) QueryLatest(ctx context.Context, channelID, fieldName string) (*query.LatestReading, error) {
	if channelID == "" {
		return nil, query.ErrInvalidChannelID
	}

	cacheKey := fmt.Sprintf("latest:%s:%s", channelID, fieldName)

	if cached, err := s.cache.Get(ctx, cacheKey); err == nil {
		var latest query.LatestReading
		if json.Unmarshal(cached, &latest) == nil {
			return &latest, nil
		}
	}

	latest, err := s.reader.QueryLatest(ctx, channelID, fieldName)
	if err != nil {
		return nil, err
	}

	if b, err := json.Marshal(latest); err == nil {
		if err := s.cache.Set(ctx, cacheKey, b, s.latestCacheTTL); err != nil {
			s.logger.Warn("failed to cache latest result", "error", err)
		}
	}

	return latest, nil
}
