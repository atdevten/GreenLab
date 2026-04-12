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

func (m *mockSchemaACKStore) IsForceDeprecated(ctx context.Context, channelID string) (bool, error) {
	args := m.Called(ctx, channelID)
	return args.Bool(0), args.Error(1)
}

func (m *mockSchemaACKStore) SetStuck(ctx context.Context, channelID, deviceID string) error {
	return m.Called(ctx, channelID, deviceID).Error(0)
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
		ChannelID:     "42",
		Fields:        []domain.FieldEntry{{Index: 1, Name: "temperature", Type: "float"}},
		SchemaVersion: 3,
	}

	svc := &mockIngestService{}
	ackStore := &mockSchemaACKStore{}

	gin.SetMode(gin.TestMode)
	h := NewHandler(svc, slog.Default(), ackStore)
	r := buildRouterWithACK(h, schema)

	svc.On("IngestBatch", mock.Anything, mock.Anything).Return(nil)
	ackStore.On("RecordACK", mock.Anything, "42", "dev-uuid-1", uint32(3)).Return(nil)

	body := []byte(`{"f":[42.5],"sv":3}`)
	w := doRequest(r, "POST", "/v1/channels/42/data", body, ctOJSON)

	require.Equal(t, http.StatusCreated, w.Code)
	svc.AssertExpectations(t)
	ackStore.AssertExpectations(t)
}

func TestHandler_IngestCompact_ACKStoreError_DoesNotFailRequest(t *testing.T) {
	schema := domain.DeviceSchema{
		DeviceID:      "dev-uuid-1",
		ChannelID:     "42",
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
	w := doRequest(r, "POST", "/v1/channels/42/data", body, ctOJSON)

	// Request must still return 201 despite ACK store failure.
	assert.Equal(t, http.StatusCreated, w.Code)
	svc.AssertExpectations(t)
	ackStore.AssertExpectations(t)
}

func TestHandler_IngestCompact_NilACKStore_DoesNotPanic(t *testing.T) {
	schema := domain.DeviceSchema{
		DeviceID:      "dev-uuid-1",
		ChannelID:     "42",
		Fields:        []domain.FieldEntry{{Index: 1, Name: "temperature", Type: "float"}},
		SchemaVersion: 1,
	}

	svc := &mockIngestService{}
	// Nil ackStore — should be a no-op.
	h := NewHandler(svc, slog.Default(), nil)
	r := buildRouterWithACK(h, schema)

	svc.On("IngestBatch", mock.Anything, mock.Anything).Return(nil)

	body := []byte(`{"f":[42.5],"sv":1}`)
	w := doRequest(r, "POST", "/v1/channels/42/data", body, ctOJSON)

	assert.Equal(t, http.StatusCreated, w.Code)
	svc.AssertExpectations(t)
}

func TestHandler_IngestCompact_ACKNotCalled_OnServiceError(t *testing.T) {
	schema := domain.DeviceSchema{
		DeviceID:      "dev-uuid-1",
		ChannelID:     "42",
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
	w := doRequest(r, "POST", "/v1/channels/42/data", body, ctOJSON)

	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
	ackStore.AssertNotCalled(t, "RecordACK")
	svc.AssertExpectations(t)
}

func TestHandler_IngestCompact_ACKUsesSchemaVersion(t *testing.T) {
	schema := domain.DeviceSchema{
		DeviceID:      "dev-uuid-99",
		ChannelID:     "99",
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
	ackStore.On("RecordACK", mock.Anything, "99", "dev-uuid-99", uint32(7)).Return(nil)

	body := []byte(`{"f":[1.0],"sv":7}`)
	w := doRequest(r, "POST", "/v1/channels/99/data", body, ctOJSON)

	require.Equal(t, http.StatusCreated, w.Code)

	var resp struct {
		Data IngestResponse `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, 1, resp.Data.Accepted)

	svc.AssertExpectations(t)
	ackStore.AssertExpectations(t)
}

// Force-deprecation tests: the 410 path is triggered when a device sends an OLD schema
// version (causing SchemaMismatchError) AND the channel has a force-deprecation marker.
// Devices already on the current schema version are not affected.

func TestHandler_IngestCompact_ForceDeprecated_OldVersion_Returns410(t *testing.T) {
	// Channel is on schema version 2; device sends sv:1 (old version).
	schema := domain.DeviceSchema{
		DeviceID:      "dev-uuid-1",
		ChannelID:     "42",
		Fields:        []domain.FieldEntry{{Index: 1, Name: "temperature", Type: "float"}},
		SchemaVersion: 2,
	}

	svc := &mockIngestService{}
	ackStore := &mockSchemaACKStore{}

	gin.SetMode(gin.TestMode)
	h := NewHandler(svc, slog.Default(), ackStore)
	r := buildRouterWithACK(h, schema)

	ackStore.On("IsForceDeprecated", mock.Anything, "42").Return(true, nil)
	ackStore.On("SetStuck", mock.Anything, "42", "dev-uuid-1").Return(nil)

	// sv:1 in payload does not match SchemaVersion:2 in auth context → SchemaMismatchError.
	body := []byte(`{"f":[42.5],"sv":1}`)
	w := doRequest(r, "POST", "/v1/channels/42/data", body, ctOJSON)

	assert.Equal(t, http.StatusGone, w.Code)
	// IngestBatch and RecordACK must not be called.
	svc.AssertNotCalled(t, "IngestBatch")
	ackStore.AssertNotCalled(t, "RecordACK")
	ackStore.AssertExpectations(t)
}

func TestHandler_IngestCompact_ForceDeprecated_CurrentVersion_NotBlocked(t *testing.T) {
	// Channel is force-deprecated, but this device is already on the current schema (v2).
	// It must NOT receive 410.
	schema := domain.DeviceSchema{
		DeviceID:      "dev-uuid-1",
		ChannelID:     "42",
		Fields:        []domain.FieldEntry{{Index: 1, Name: "temperature", Type: "float"}},
		SchemaVersion: 2,
	}

	svc := &mockIngestService{}
	ackStore := &mockSchemaACKStore{}

	gin.SetMode(gin.TestMode)
	h := NewHandler(svc, slog.Default(), ackStore)
	r := buildRouterWithACK(h, schema)

	// sv:2 in payload matches SchemaVersion:2 — no SchemaMismatchError, force-deprecation
	// check is never reached.
	svc.On("IngestBatch", mock.Anything, mock.Anything).Return(nil)
	ackStore.On("IsForceDeprecated", mock.Anything, mock.Anything).Maybe().Return(false, nil)
	ackStore.On("RecordACK", mock.Anything, "42", "dev-uuid-1", uint32(2)).Return(nil)

	body := []byte(`{"f":[42.5],"sv":2}`)
	w := doRequest(r, "POST", "/v1/channels/42/data", body, ctOJSON)

	assert.Equal(t, http.StatusCreated, w.Code)
	svc.AssertExpectations(t)
	ackStore.AssertNotCalled(t, "SetStuck")
}

func TestHandler_IngestCompact_ForceDeprecated_SetStuckError_StillReturns410(t *testing.T) {
	// SetStuck fails — 410 should still be returned (fail-open on stuck marking).
	schema := domain.DeviceSchema{
		DeviceID:      "dev-uuid-1",
		ChannelID:     "42",
		Fields:        []domain.FieldEntry{{Index: 1, Name: "temperature", Type: "float"}},
		SchemaVersion: 2,
	}

	svc := &mockIngestService{}
	ackStore := &mockSchemaACKStore{}

	gin.SetMode(gin.TestMode)
	h := NewHandler(svc, slog.Default(), ackStore)
	r := buildRouterWithACK(h, schema)

	ackStore.On("IsForceDeprecated", mock.Anything, "42").Return(true, nil)
	ackStore.On("SetStuck", mock.Anything, "42", "dev-uuid-1").Return(errors.New("redis unavailable"))

	body := []byte(`{"f":[42.5],"sv":1}`)
	w := doRequest(r, "POST", "/v1/channels/42/data", body, ctOJSON)

	assert.Equal(t, http.StatusGone, w.Code)
	svc.AssertNotCalled(t, "IngestBatch")
	ackStore.AssertExpectations(t)
}

func TestHandler_IngestCompact_ForceDeprecatedCheckError_FailsOpen(t *testing.T) {
	// IsForceDeprecated returns an error — should fail-open and return normal 409.
	schema := domain.DeviceSchema{
		DeviceID:      "dev-uuid-1",
		ChannelID:     "42",
		Fields:        []domain.FieldEntry{{Index: 1, Name: "temperature", Type: "float"}},
		SchemaVersion: 2,
	}

	svc := &mockIngestService{}
	ackStore := &mockSchemaACKStore{}

	gin.SetMode(gin.TestMode)
	h := NewHandler(svc, slog.Default(), ackStore)
	r := buildRouterWithACK(h, schema)

	ackStore.On("IsForceDeprecated", mock.Anything, "42").Return(false, errors.New("redis timeout"))

	// Device sends old sv:1 — schema mismatch, but deprecation check fails open → 409.
	body := []byte(`{"f":[42.5],"sv":1}`)
	w := doRequest(r, "POST", "/v1/channels/42/data", body, ctOJSON)

	assert.Equal(t, http.StatusConflict, w.Code) // 409, not 410 (fail-open)
	svc.AssertNotCalled(t, "IngestBatch")
	ackStore.AssertNotCalled(t, "SetStuck")
	ackStore.AssertExpectations(t)
}
