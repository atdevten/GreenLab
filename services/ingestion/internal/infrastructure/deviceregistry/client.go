package deviceregistry

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"

	"github.com/greenlab/ingestion/internal/domain"
)

// validateAPIKeyRequest mirrors the device-registry internal endpoint request.
type validateAPIKeyRequest struct {
	APIKey    string `json:"api_key"`
	ChannelID string `json:"channel_id"`
}

// fieldEntryJSON mirrors the device-registry internal endpoint field response.
type fieldEntryJSON struct {
	Index uint8  `json:"index"`
	Name  string `json:"name"`
	Type  string `json:"type"`
}

// validateAPIKeyResponse mirrors the device-registry internal endpoint response body.
type validateAPIKeyResponse struct {
	DeviceID      string           `json:"device_id"`
	Fields        []fieldEntryJSON `json:"fields"`
	SchemaVersion uint32           `json:"schema_version"`
}

// apiResponse wraps the shared response envelope used by device-registry.
type apiResponse struct {
	Data validateAPIKeyResponse `json:"data"`
}

// Client calls device-registry's internal validate-api-key endpoint.
type Client struct {
	baseURL    string
	httpClient *http.Client
}

// NewClient creates an HTTP client for device-registry.
// baseURL should be the base URL of the device-registry service, e.g. "http://device-registry:8002".
func NewClient(baseURL string, httpClient *http.Client) *Client {
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	return &Client{baseURL: baseURL, httpClient: httpClient}
}

// GetByAPIKey implements the store interface for apikey.Validator.
// Returns domain.ErrDeviceNotFound when device-registry returns 401.
func (c *Client) GetByAPIKey(ctx context.Context, apiKey, channelID string) (domain.DeviceSchema, error) {
	body, err := json.Marshal(validateAPIKeyRequest{APIKey: apiKey, ChannelID: channelID})
	if err != nil {
		return domain.DeviceSchema{}, fmt.Errorf("Client.GetByAPIKey marshal: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		c.baseURL+"/internal/validate-api-key", bytes.NewReader(body))
	if err != nil {
		return domain.DeviceSchema{}, fmt.Errorf("Client.GetByAPIKey new request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return domain.DeviceSchema{}, fmt.Errorf("Client.GetByAPIKey http: %w", err)
	}
	defer resp.Body.Close() //nolint:errcheck

	if resp.StatusCode == http.StatusUnauthorized {
		return domain.DeviceSchema{}, fmt.Errorf("Client.GetByAPIKey: %w", domain.ErrDeviceNotFound)
	}

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return domain.DeviceSchema{}, fmt.Errorf("Client.GetByAPIKey unexpected status %d: %s", resp.StatusCode, b)
	}

	var envelope apiResponse
	if err := json.NewDecoder(resp.Body).Decode(&envelope); err != nil {
		return domain.DeviceSchema{}, fmt.Errorf("Client.GetByAPIKey decode: %w", err)
	}

	r := envelope.Data
	if r.DeviceID == "" {
		return domain.DeviceSchema{}, errors.New("Client.GetByAPIKey: empty device_id in response")
	}

	fields := make([]domain.FieldEntry, len(r.Fields))
	for i, f := range r.Fields {
		fields[i] = domain.FieldEntry{Index: f.Index, Name: f.Name, Type: f.Type}
	}

	return domain.DeviceSchema{
		DeviceID:      r.DeviceID,
		ChannelID:     channelID,
		Fields:        fields,
		SchemaVersion: r.SchemaVersion,
	}, nil
}
