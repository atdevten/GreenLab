package sdk

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

// fieldEntry describes a single field in the channel schema.
type fieldEntry struct {
	Index uint8  `json:"index"`
	Name  string `json:"name"`
	Type  string `json:"type"`
}

// schemaData is the inner schema payload from the API envelope.
type schemaData struct {
	Fields        []fieldEntry `json:"fields"`
	SchemaVersion uint32       `json:"schema_version"`
}

// schemaEnvelope matches the GreenLab standard API envelope:
// {"success": true, "data": {...}}
type schemaEnvelope struct {
	Success bool       `json:"success"`
	Data    schemaData `json:"data"`
}

// channelSchema caches the resolved schema for a channel.
type channelSchema struct {
	// nameToIndex maps field name → 1-based positional index in the f array.
	// The server returns fields ordered by index, so position i in the f array
	// corresponds to the field with Index == i+1.
	nameToIndex   map[string]uint8
	indexToName   map[uint8]string
	schemaVersion uint32
	// orderedFields holds the fields sorted by index for building the f array.
	orderedFields []fieldEntry
}

// fetchSchema calls GET {baseURL}/v1/channels/{channelID}/schema and builds
// the local schema cache. It sets X-API-Key on the request.
func fetchSchema(ctx context.Context, baseURL, channelID, apiKey string) (channelSchema, error) {
	url := fmt.Sprintf("%s/v1/channels/%s/schema", baseURL, channelID)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return channelSchema{}, fmt.Errorf("sdk: build schema request: %w", err)
	}
	req.Header.Set("X-API-Key", apiKey)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return channelSchema{}, fmt.Errorf("sdk: schema request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return channelSchema{}, fmt.Errorf("sdk: schema fetch returned status %d", resp.StatusCode)
	}

	var env schemaEnvelope
	if err := json.NewDecoder(resp.Body).Decode(&env); err != nil {
		return channelSchema{}, fmt.Errorf("sdk: decode schema response: %w", err)
	}
	if !env.Success {
		return channelSchema{}, fmt.Errorf("sdk: schema response reported failure")
	}

	return buildChannelSchema(env.Data), nil
}

// buildChannelSchema converts the raw API schema into the internal cache.
func buildChannelSchema(data schemaData) channelSchema {
	cs := channelSchema{
		nameToIndex:   make(map[string]uint8, len(data.Fields)),
		indexToName:   make(map[uint8]string, len(data.Fields)),
		schemaVersion: data.SchemaVersion,
		orderedFields: data.Fields,
	}
	for _, f := range data.Fields {
		cs.nameToIndex[f.Name] = f.Index
		cs.indexToName[f.Index] = f.Name
	}
	return cs
}
