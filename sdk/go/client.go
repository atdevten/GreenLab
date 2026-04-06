package sdk

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"
)

const (
	defaultBatchSize    = 10
	retryTimeout        = 5 * time.Second
	contentTypeMsgPack  = "application/msgpack"
	headerContentEnc    = "Content-Encoding"
	headerRecommFmt     = "X-Recommended-Format"
	encodingLZ4         = "lz4"
)

// Config holds SDK configuration.
type Config struct {
	// BaseURL is the ingestion service base URL, e.g. "http://localhost:8003".
	BaseURL string
	// APIKey is the device write API key.
	APIKey string
	// ChannelID is the UUID of the channel to write to.
	ChannelID string
	// BatchSize is the maximum number of readings per Send call (default 10).
	BatchSize int
	// ConfigFile is the path for local persistence. Defaults to "~/.greenlab/sdk.json".
	ConfigFile string
}

// Client sends telemetry readings to a GreenLab ingestion endpoint.
type Client struct {
	cfg        Config
	schema     channelSchema
	localCfg   localConfig
	mu         sync.Mutex
	pending    []reading
	httpClient *http.Client
}

// New creates a Client, fetches the channel schema, and initialises local config.
func New(cfg Config) (*Client, error) {
	if cfg.BaseURL == "" {
		return nil, fmt.Errorf("sdk: BaseURL is required")
	}
	if cfg.APIKey == "" {
		return nil, fmt.Errorf("sdk: APIKey is required")
	}
	if cfg.ChannelID == "" {
		return nil, fmt.Errorf("sdk: ChannelID is required")
	}
	if cfg.BatchSize <= 0 {
		cfg.BatchSize = defaultBatchSize
	}
	if cfg.ConfigFile == "" {
		cfg.ConfigFile = defaultConfigFile
	}

	c := &Client{
		cfg:        cfg,
		httpClient: &http.Client{Timeout: 30 * time.Second},
	}

	// Load persisted config (best-effort; missing file is not an error).
	lc, err := loadConfig(cfg.ConfigFile)
	if err == nil {
		c.localCfg = lc
	}

	// Fetch schema from server.
	schema, err := fetchSchema(context.Background(), cfg.BaseURL, cfg.ChannelID, cfg.APIKey)
	if err != nil {
		return nil, err
	}
	c.schema = schema

	// Persist updated config.
	c.localCfg.ChannelID = cfg.ChannelID
	c.localCfg.SchemaVersion = schema.schemaVersion
	if c.localCfg.Format == "" {
		c.localCfg.Format = "msgpack"
	}
	_ = saveConfig(cfg.ConfigFile, c.localCfg)

	return c, nil
}

// SetField records a single field value at the current time.
func (c *Client) SetField(name string, value float64) {
	c.SetFieldAt(name, value, time.Now().UTC())
}

// SetFieldAt records a single field value at a specific time.
func (c *Client) SetFieldAt(name string, value float64, t time.Time) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.pending = append(c.pending, reading{
		fieldName: name,
		value:     value,
		ts:        t,
	})
}

// Send flushes the current batch to the server.
// Returns nil on success. Handles 409 by re-fetching schema and retrying once
// within retryTimeout (5 seconds).
func (c *Client) Send(ctx context.Context) error {
	c.mu.Lock()
	toSend := c.pending
	c.pending = nil
	c.mu.Unlock()

	if len(toSend) == 0 {
		return nil
	}

	err := c.sendReadings(ctx, toSend)
	if err == nil {
		return nil
	}

	// On schema mismatch (409), re-fetch schema once and retry.
	if isSchemaMismatch(err) {
		retryCtx, cancel := context.WithTimeout(ctx, retryTimeout)
		defer cancel()

		schema, fetchErr := fetchSchema(retryCtx, c.cfg.BaseURL, c.cfg.ChannelID, c.cfg.APIKey)
		if fetchErr != nil {
			return fmt.Errorf("sdk: schema re-fetch after 409: %w", fetchErr)
		}

		c.mu.Lock()
		c.schema = schema
		c.localCfg.SchemaVersion = schema.schemaVersion
		c.mu.Unlock()
		_ = saveConfig(c.cfg.ConfigFile, c.localCfg)

		return c.sendReadings(retryCtx, toSend)
	}

	return err
}

// sendReadings encodes the readings into one or more msgpack batches and posts them.
func (c *Client) sendReadings(ctx context.Context, readings []reading) error {
	c.mu.Lock()
	schema := c.schema
	schemaVersion := c.localCfg.SchemaVersion
	c.mu.Unlock()

	payloads, err := buildBatches(readings, schema, schemaVersion)
	if err != nil {
		return err
	}

	for _, payload := range payloads {
		if err := c.postPayload(ctx, payload); err != nil {
			return err
		}
	}
	return nil
}

// postPayload sends a single MessagePack payload to the ingestion endpoint,
// applying LZ4 compression if the payload exceeds the threshold.
func (c *Client) postPayload(ctx context.Context, payload []byte) error {
	compressed, didCompress, err := maybeCompress(payload)
	if err != nil {
		return err
	}

	url := fmt.Sprintf("%s/v1/channels/%s/data", c.cfg.BaseURL, c.cfg.ChannelID)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(compressed))
	if err != nil {
		return fmt.Errorf("sdk: build request: %w", err)
	}
	req.Header.Set("Content-Type", contentTypeMsgPack)
	req.Header.Set("X-API-Key", c.cfg.APIKey)
	if didCompress {
		req.Header.Set(headerContentEnc, encodingLZ4)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("sdk: post readings: %w", err)
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, resp.Body)

	// Process X-Recommended-Format header.
	if recFmt := resp.Header.Get(headerRecommFmt); recFmt != "" {
		c.mu.Lock()
		c.localCfg.Format = recFmt
		c.mu.Unlock()
		_ = saveConfig(c.cfg.ConfigFile, c.localCfg)
	}

	if resp.StatusCode == http.StatusConflict {
		return errSchemaMismatch
	}
	if resp.StatusCode >= 300 {
		return fmt.Errorf("sdk: unexpected status %d", resp.StatusCode)
	}
	return nil
}

// errSchemaMismatch is returned when the server responds with HTTP 409.
var errSchemaMismatch = fmt.Errorf("sdk: schema version mismatch (409)")

// isSchemaMismatch reports whether err is (or wraps) errSchemaMismatch.
func isSchemaMismatch(err error) bool {
	return err != nil && err.Error() == errSchemaMismatch.Error()
}
