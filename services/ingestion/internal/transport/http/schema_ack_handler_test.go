package http

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/greenlab/ingestion/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// mockSchemaACKStore mocks the schemaACKStore interface.
type mockSchemaACKStore struct{ mock.Mock }

func (m *mockSchemaACKStore) RecordACK(ctx context.Context, channelID, deviceID string, version uint32) error {
	return m.Called(ctx, channelID, deviceID, version).Error(0)
}

func buildRouterWithACK(h *Handler, schema domain.DeviceSchema) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("device_schema", schema)
		c.Set("device_id", schema.DeviceID)
		c.Set("request_id", testRequestID)
		c.Next()
	})
	r.POST("/v1/channels/:channel_id/data", h.Ingest)
	return r
}

func TestHandler_IngestCompact_RecordsACK_OnSuccess(t *testing.T) {
	schema := domain.DeviceSchema{
		DeviceID:      "dev-uuid-1",
		ChannelID:     "chan-uuid-1",
		Fields:        []domain.FieldEntry{{Index: 1, Name: "temperature", Type: "float"}},
		SchemaVersion: 3,
	}

	svc := &mockIngestService{}
	ackStore := &mockSchemaACKStore{}

	gin.SetMode(gin.TestMode)
	h := NewHandler(svc, slog.Default(), ackStore)
	r := buildRouterWithACK(h, schema)

	svc.On("IngestBatch", mock.Anything, mock.Anything).Return(nil)
	ackStore.On("RecordACK", mock.Anything, "chan-uuid-1", "dev-uuid-1", uint32(3)).Return(nil)

	body := []byte(`{"f":[42.5],"sv":3}`)
	w := doRequest(r, "POST", "/v1/channels/chan-uuid-1/data", body, ctOJSON)

	require.Equal(t, http.StatusCreated, w.Code)
	svc.AssertExpectations(t)
	ackStore.AssertExpectations(t)
}

func TestHandler_IngestCompact_ACKStoreError_DoesNotFailRequest(t *testing.T) {
	schema := domain.DeviceSchema{
		DeviceID:      "dev-uuid-1",
		ChannelID:     "chan-uuid-1",
		Fields:        []domain.FieldEntry{{Index: 1, Name: "temperature", Type: "float"}},
		SchemaVersion: 2,
	}

	svc := &mockIngestService{}
	ackStore := &mockSchemaACKStore{}

	gin.SetMode(gin.TestMode)
	h := NewHandler(svc, slog.Default(), ackStore)
	r := buildRouterWithACK(h, schema)

	svc.On("IngestBatch", mock.Anything, mock.Anything).Return(nil)
	// ACK store returns an error — request should still succeed (fail-open).
	ackStore.On("RecordACK", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(errors.New("redis connection refused"))

	body := []byte(`{"f":[42.5],"sv":2}`)
	w := doRequest(r, "POST", "/v1/channels/chan-uuid-1/data", body, ctOJSON)

	// Request must still return 201 despite ACK store failure.
	assert.Equal(t, http.StatusCreated, w.Code)
	svc.AssertExpectations(t)
	ackStore.AssertExpectations(t)
}

func TestHandler_IngestCompact_NilACKStore_DoesNotPanic(t *testing.T) {
	schema := domain.DeviceSchema{
		DeviceID:      "dev-uuid-1",
		ChannelID:     "chan-uuid-1",
		Fields:        []domain.FieldEntry{{Index: 1, Name: "temperature", Type: "float"}},
		SchemaVersion: 1,
	}

	svc := &mockIngestService{}
	// Nil ackStore — should be a no-op.
	h := NewHandler(svc, slog.Default(), nil)
	r := buildRouterWithACK(h, schema)

	svc.On("IngestBatch", mock.Anything, mock.Anything).Return(nil)

	body := []byte(`{"f":[42.5],"sv":1}`)
	w := doRequest(r, "POST", "/v1/channels/chan-uuid-1/data", body, ctOJSON)

	assert.Equal(t, http.StatusCreated, w.Code)
	svc.AssertExpectations(t)
}

func TestHandler_IngestCompact_ACKNotCalled_OnServiceError(t *testing.T) {
	schema := domain.DeviceSchema{
		DeviceID:      "dev-uuid-1",
		ChannelID:     "chan-uuid-1",
		Fields:        []domain.FieldEntry{{Index: 1, Name: "temperature", Type: "float"}},
		SchemaVersion: 1,
	}

	svc := &mockIngestService{}
	ackStore := &mockSchemaACKStore{}

	gin.SetMode(gin.TestMode)
	h := NewHandler(svc, slog.Default(), ackStore)
	r := buildRouterWithACK(h, schema)

	// IngestBatch fails — ACK must not be recorded.
	svc.On("IngestBatch", mock.Anything, mock.Anything).Return(errors.New("kafka unavailable"))

	body := []byte(`{"f":[42.5],"sv":1}`)
	w := doRequest(r, "POST", "/v1/channels/chan-uuid-1/data", body, ctOJSON)

	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
	ackStore.AssertNotCalled(t, "RecordACK")
	svc.AssertExpectations(t)
}

func TestHandler_IngestCompact_ACKUsesSchemaVersion(t *testing.T) {
	schema := domain.DeviceSchema{
		DeviceID:      "dev-uuid-99",
		ChannelID:     "chan-uuid-99",
		Fields:        []domain.FieldEntry{{Index: 1, Name: "temp", Type: "float"}},
		SchemaVersion: 7,
	}

	svc := &mockIngestService{}
	ackStore := &mockSchemaACKStore{}

	gin.SetMode(gin.TestMode)
	h := NewHandler(svc, slog.Default(), ackStore)
	r := buildRouterWithACK(h, schema)

	svc.On("IngestBatch", mock.Anything, mock.Anything).Return(nil)
	// Verify schema version 7 is passed to RecordACK, not the payload's "sv" field.
	ackStore.On("RecordACK", mock.Anything, "chan-uuid-99", "dev-uuid-99", uint32(7)).Return(nil)

	body := []byte(`{"f":[1.0],"sv":7}`)
	w := doRequest(r, "POST", "/v1/channels/chan-uuid-99/data", body, ctOJSON)

	require.Equal(t, http.StatusCreated, w.Code)

	var resp struct {
		Data IngestResponse `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, 1, resp.Data.Accepted)

	svc.AssertExpectations(t)
	ackStore.AssertExpectations(t)
}
