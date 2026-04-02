package influxdb

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/greenlab/device-registry/internal/application"
)

// bucketEntry represents a single item from GET /api/v2/buckets.
type bucketEntry struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// bucketListFull is the full response from GET /api/v2/buckets.
type bucketListFull struct {
	Buckets []bucketEntry `json:"buckets"`
}

// GetStorageUsage returns the list of channel buckets and their reported sizes.
// InfluxDB OSS does not expose per-bucket byte counts via a public API, so
// SizeBytes is always 0 in OSS deployments. The method still returns the
// channel bucket list, which is useful for operators.
func (m *RetentionManager) GetStorageUsage(ctx context.Context) ([]application.BucketUsage, error) {
	url := fmt.Sprintf("%s/api/v2/buckets?org=%s&limit=500", m.cfg.URL, m.cfg.Org)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("GetStorageUsage.NewRequest: %w", err)
	}
	req.Header.Set("Authorization", "Token "+m.cfg.Token)

	resp, err := m.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("GetStorageUsage.Do: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("GetStorageUsage: unexpected status %d: %s", resp.StatusCode, string(body))
	}

	var result bucketListFull
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("GetStorageUsage.Decode: %w", err)
	}

	// Filter to channel buckets only (name starts with "channel-").
	usage := make([]application.BucketUsage, 0, len(result.Buckets))
	for _, b := range result.Buckets {
		if len(b.Name) > 8 && b.Name[:8] == "channel-" {
			usage = append(usage, application.BucketUsage{
				BucketID:   b.ID,
				BucketName: b.Name,
				SizeBytes:  0, // InfluxDB OSS does not expose per-bucket sizes
			})
		}
	}
	return usage, nil
}
