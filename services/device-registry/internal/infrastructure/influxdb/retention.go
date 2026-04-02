package influxdb

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// Config holds the InfluxDB connection settings.
type Config struct {
	URL   string
	Token string
	Org   string
}

// RetentionManager manages InfluxDB bucket retention policies via the HTTP API.
type RetentionManager struct {
	cfg    Config
	client *http.Client
}

// NewRetentionManager creates a new RetentionManager.
func NewRetentionManager(cfg Config) *RetentionManager {
	return &RetentionManager{
		cfg:    cfg,
		client: &http.Client{Timeout: 10 * time.Second},
	}
}

// SetRetention creates or updates an InfluxDB bucket named "channel-{channelID}"
// with a retention duration of days*24h.
func (m *RetentionManager) SetRetention(ctx context.Context, channelID string, days int) error {
	bucketName := fmt.Sprintf("channel-%s", channelID)
	durationNs := int64(days) * 24 * int64(time.Hour)

	// Try to find the existing bucket first.
	bucketID, err := m.findBucket(ctx, bucketName)
	if err != nil {
		return fmt.Errorf("RetentionManager.SetRetention: %w", err)
	}

	if bucketID != "" {
		return m.updateBucket(ctx, bucketID, durationNs)
	}
	return m.createBucket(ctx, bucketName, durationNs)
}

// bucketListResponse is the JSON shape returned by GET /api/v2/buckets.
type bucketListResponse struct {
	Buckets []struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	} `json:"buckets"`
}

func (m *RetentionManager) findBucket(ctx context.Context, name string) (string, error) {
	url := fmt.Sprintf("%s/api/v2/buckets?name=%s&org=%s", m.cfg.URL, name, m.cfg.Org)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", fmt.Errorf("findBucket.NewRequest: %w", err)
	}
	req.Header.Set("Authorization", "Token "+m.cfg.Token)

	resp, err := m.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("findBucket.Do: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("findBucket: unexpected status %d: %s", resp.StatusCode, string(body))
	}

	var result bucketListResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("findBucket.Decode: %w", err)
	}

	for _, b := range result.Buckets {
		if b.Name == name {
			return b.ID, nil
		}
	}
	return "", nil
}

// createBucketRequest is the JSON body for POST /api/v2/buckets.
type createBucketRequest struct {
	OrgID          string               `json:"orgID,omitempty"`
	Org            string               `json:"org,omitempty"`
	Name           string               `json:"name"`
	RetentionRules []retentionRuleEntry `json:"retentionRules"`
}

type retentionRuleEntry struct {
	Type         string `json:"type"`
	EverySeconds int64  `json:"everySeconds"`
}

func (m *RetentionManager) createBucket(ctx context.Context, name string, durationNs int64) error {
	everySeconds := durationNs / int64(time.Second)
	body := createBucketRequest{
		Org:  m.cfg.Org,
		Name: name,
		RetentionRules: []retentionRuleEntry{
			{Type: "expire", EverySeconds: everySeconds},
		},
	}

	data, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("createBucket.Marshal: %w", err)
	}

	url := fmt.Sprintf("%s/api/v2/buckets", m.cfg.URL)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("createBucket.NewRequest: %w", err)
	}
	req.Header.Set("Authorization", "Token "+m.cfg.Token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := m.client.Do(req)
	if err != nil {
		return fmt.Errorf("createBucket.Do: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("createBucket: unexpected status %d: %s", resp.StatusCode, string(body))
	}
	return nil
}

// updateBucketRequest is the JSON body for PATCH /api/v2/buckets/{id}.
type updateBucketRequest struct {
	RetentionRules []retentionRuleEntry `json:"retentionRules"`
}

func (m *RetentionManager) updateBucket(ctx context.Context, bucketID string, durationNs int64) error {
	everySeconds := durationNs / int64(time.Second)
	body := updateBucketRequest{
		RetentionRules: []retentionRuleEntry{
			{Type: "expire", EverySeconds: everySeconds},
		},
	}

	data, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("updateBucket.Marshal: %w", err)
	}

	url := fmt.Sprintf("%s/api/v2/buckets/%s", m.cfg.URL, bucketID)
	req, err := http.NewRequestWithContext(ctx, http.MethodPatch, url, bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("updateBucket.NewRequest: %w", err)
	}
	req.Header.Set("Authorization", "Token "+m.cfg.Token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := m.client.Do(req)
	if err != nil {
		return fmt.Errorf("updateBucket.Do: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("updateBucket: unexpected status %d: %s", resp.StatusCode, string(body))
	}
	return nil
}
