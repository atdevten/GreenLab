package redis

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/redis/go-redis/v9"
	"github.com/greenlab/ingestion/internal/application"
)

const replayFailureMetricKey = "metric:replay_publish_failure_total"

// ReplayDLQ writes failed replay readings to a Redis list keyed by channel ID.
type ReplayDLQ struct {
	client *redis.Client
}

// NewReplayDLQ creates a ReplayDLQ backed by the given Redis client.
func NewReplayDLQ(client *redis.Client) *ReplayDLQ {
	return &ReplayDLQ{client: client}
}

// dlqKey returns the Redis list key for a channel's DLQ.
func dlqKey(channelID string) string {
	return fmt.Sprintf("replay_dlq:%s", channelID)
}

// Push appends a DLQ entry to the channel's replay DLQ list.
// No TTL is set; the list persists until an operator clears it.
func (d *ReplayDLQ) Push(ctx context.Context, entry application.ReplayDLQEntry) error {
	b, err := json.Marshal(entry)
	if err != nil {
		return fmt.Errorf("ReplayDLQ.Push: marshal: %w", err)
	}
	if err := d.client.RPush(ctx, dlqKey(entry.ChannelID), b).Err(); err != nil {
		return fmt.Errorf("ReplayDLQ.Push: %w", err)
	}
	return nil
}

// IncrFailureMetric increments the global replay publish failure counter.
func (d *ReplayDLQ) IncrFailureMetric(ctx context.Context) error {
	if err := d.client.Incr(ctx, replayFailureMetricKey).Err(); err != nil {
		return fmt.Errorf("ReplayDLQ.IncrFailureMetric: %w", err)
	}
	return nil
}
