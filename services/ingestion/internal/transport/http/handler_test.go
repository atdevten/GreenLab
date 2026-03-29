package http

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/vmihailenco/msgpack/v5"
	"github.com/greenlab/ingestion/internal/application"
	"github.com/greenlab/ingestion/internal/domain"
)

// --- mock ingestService ---

type mockIngestService struct{ mock.Mock }

func (m *mockIngestService) Ingest(ctx context.Context, in application.IngestInput) error {
	return m.Called(ctx, in).Error(0)
}

func (m *mockIngestService) IngestBatch(ctx context.Context, readings []application.IngestInput) error {
	return m.Called(ctx, readings).Error(0)
}

// --- test helpers ---

func newTestHandler(svc ingestService) *Handler {
	gin.SetMode(gin.TestMode)
	return NewHandler(svc, slog.Default())
}

func buildRouter(h *Handler, schema domain.DeviceSchema) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("device_schema", schema)
		c.Set("device_id", schema.DeviceID)
		c.Next()
	})
	r.POST("/v1/channels/:channel_id/data", h.Ingest)
	r.POST("/v1/channels/:channel_id/data/bulk", h.BulkIngest)
	return r
}

func doRequest(r *gin.Engine, method, path string, body []byte, contentType string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, bytes.NewReader(body))
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

var testSchema = domain.DeviceSchema{
	DeviceID:      "dev-uuid-1",
	ChannelID:     uuid.New().String(),
	Fields:        []domain.FieldEntry{{Index: 1, Name: "temperature", Type: "float"}},
	SchemaVersion: 1,
}

// --- tests ---

func TestHandler_Ingest_JSON(t *testing.T) {
	channelID := uuid.New().String()

	t.Run("application/json — existing behavior unchanged", func(t *testing.T) {
		svc := &mockIngestService{}
		h := newTestHandler(svc)
		r := buildRouter(h, testSchema)

		svc.On("Ingest", mock.Anything, mock.MatchedBy(func(in application.IngestInput) bool {
			return in.ChannelID == channelID && in.DeviceID == testSchema.DeviceID
		})).Return(nil)

		body, _ := json.Marshal(map[string]any{"fields": map[string]float64{"temp": 22.5}})
		w := doRequest(r, "POST", "/v1/channels/"+channelID+"/data", body, "application/json")
		assert.Equal(t, http.StatusCreated, w.Code)
		svc.AssertExpectations(t)
	})

	t.Run("empty Content-Type (default) also uses JSON path", func(t *testing.T) {
		svc := &mockIngestService{}
		h := newTestHandler(svc)
		r := buildRouter(h, testSchema)

		svc.On("Ingest", mock.Anything, mock.Anything).Return(nil)

		body, _ := json.Marshal(map[string]any{"fields": map[string]float64{"temp": 22.5}})
		w := doRequest(r, "POST", "/v1/channels/"+channelID+"/data", body, "")
		assert.Equal(t, http.StatusCreated, w.Code)
		svc.AssertExpectations(t)
	})

	t.Run("missing fields in JSON — 422 (validation error)", func(t *testing.T) {
		svc := &mockIngestService{}
		h := newTestHandler(svc)
		r := buildRouter(h, testSchema)

		body, _ := json.Marshal(map[string]any{"tags": map[string]string{"x": "y"}})
		w := doRequest(r, "POST", "/v1/channels/"+channelID+"/data", body, "application/json")
		assert.Equal(t, http.StatusUnprocessableEntity, w.Code)
		svc.AssertNotCalled(t, "Ingest")
	})
}

func TestHandler_Ingest_OJSON(t *testing.T) {
	channelID := uuid.New().String()
	schema := domain.DeviceSchema{
		DeviceID:      "dev-uuid-1",
		ChannelID:     channelID,
		Fields:        []domain.FieldEntry{{Index: 1, Name: "temperature", Type: "float"}},
		SchemaVersion: 1,
	}

	t.Run("valid ojson payload parsed and ingested", func(t *testing.T) {
		svc := &mockIngestService{}
		h := newTestHandler(svc)
		r := buildRouter(h, schema)

		svc.On("IngestBatch", mock.Anything, mock.MatchedBy(func(inputs []application.IngestInput) bool {
			return len(inputs) == 1 &&
				inputs[0].ChannelID == channelID &&
				inputs[0].Fields["temperature"] == 42.5
		})).Return(nil)

		body := []byte(`{"f":[42.5],"sv":1}`)
		w := doRequest(r, "POST", "/v1/channels/"+channelID+"/data", body, ctOJSON)
		assert.Equal(t, http.StatusCreated, w.Code)
		svc.AssertExpectations(t)
	})

	t.Run("unknown field index in ojson — 400", func(t *testing.T) {
		svc := &mockIngestService{}
		h := newTestHandler(svc)
		// Schema has only field index 1; payload sends index 2.
		r := buildRouter(h, schema)

		body := []byte(`{"f":[1.0, 2.0],"sv":1}`) // index 2 not in schema
		w := doRequest(r, "POST", "/v1/channels/"+channelID+"/data", body, ctOJSON)
		assert.Equal(t, http.StatusBadRequest, w.Code)
		svc.AssertNotCalled(t, "IngestBatch")
	})
}

func TestHandler_Ingest_MsgPack(t *testing.T) {
	channelID := uuid.New().String()
	schema := domain.DeviceSchema{
		DeviceID:      "dev-uuid-1",
		ChannelID:     channelID,
		Fields:        []domain.FieldEntry{{Index: 1, Name: "temperature", Type: "float"}},
		SchemaVersion: 1,
	}

	t.Run("valid msgpack payload parsed and ingested", func(t *testing.T) {
		svc := &mockIngestService{}
		h := newTestHandler(svc)
		r := buildRouter(h, schema)

		svc.On("IngestBatch", mock.Anything, mock.MatchedBy(func(inputs []application.IngestInput) bool {
			return len(inputs) == 1 && inputs[0].Fields["temperature"] == 10.0
		})).Return(nil)

		payload := map[string]any{"f": []float64{10.0}, "sv": uint32(1)}
		body, err := msgpack.Marshal(payload)
		require.NoError(t, err)

		w := doRequest(r, "POST", "/v1/channels/"+channelID+"/data", body, ctMsgPack)
		assert.Equal(t, http.StatusCreated, w.Code)
		svc.AssertExpectations(t)
	})
}

func TestHandler_Ingest_Binary(t *testing.T) {
	authDeviceID := "6ba7b810-9dad-11d1-80b4-00c04fd430c8"
	channelID := uuid.New().String()
	schema := domain.DeviceSchema{
		DeviceID:      authDeviceID,
		ChannelID:     channelID,
		Fields:        []domain.FieldEntry{{Index: 1, Name: "temperature", Type: "float"}},
		SchemaVersion: 1,
	}

	t.Run("valid binary frame parsed and ingested", func(t *testing.T) {
		svc := &mockIngestService{}
		h := newTestHandler(svc)
		r := buildRouter(h, schema)

		svc.On("IngestBatch", mock.Anything, mock.MatchedBy(func(inputs []application.IngestInput) bool {
			return len(inputs) == 1 && inputs[0].Fields["temperature"] == float64(1000)
		})).Return(nil)

		devID := devIDFromUUID(authDeviceID)
		frame := buildBinaryFrame(t, devID, uint32(time.Now().Unix()), 0b00000001, []uint16{1000})
		w := doRequest(r, "POST", "/v1/channels/"+channelID+"/data", frame, ctBinary)
		assert.Equal(t, http.StatusCreated, w.Code)
		svc.AssertExpectations(t)
	})
}

func TestHandler_Ingest_ContentTypeDispatch(t *testing.T) {
	channelID := uuid.New().String()

	t.Run("unknown Content-Type returns 415", func(t *testing.T) {
		svc := &mockIngestService{}
		h := newTestHandler(svc)
		r := buildRouter(h, testSchema)

		w := doRequest(r, "POST", "/v1/channels/"+channelID+"/data", []byte("data"), "application/xml")
		assert.Equal(t, http.StatusUnsupportedMediaType, w.Code)
		svc.AssertNotCalled(t, "Ingest")
	})

	t.Run("protobuf Content-Type returns 501", func(t *testing.T) {
		svc := &mockIngestService{}
		h := newTestHandler(svc)
		r := buildRouter(h, testSchema)

		w := doRequest(r, "POST", "/v1/channels/"+channelID+"/data", []byte{}, ctProtobuf)
		assert.Equal(t, http.StatusNotImplemented, w.Code)
		svc.AssertNotCalled(t, "Ingest")
	})
}

func TestHandler_BulkIngest_JSON(t *testing.T) {
	channelID := uuid.New().String()

	t.Run("valid bulk ingest request", func(t *testing.T) {
		svc := &mockIngestService{}
		h := newTestHandler(svc)
		r := buildRouter(h, testSchema)

		svc.On("IngestBatch", mock.Anything, mock.MatchedBy(func(inputs []application.IngestInput) bool {
			return len(inputs) == 2
		})).Return(nil)

		body, _ := json.Marshal(map[string]any{
			"readings": []map[string]any{
				{"fields": map[string]float64{"temp": 20.0}},
				{"fields": map[string]float64{"temp": 21.0}},
			},
		})
		w := doRequest(r, "POST", "/v1/channels/"+channelID+"/data/bulk", body, "application/json")
		assert.Equal(t, http.StatusCreated, w.Code)
		svc.AssertExpectations(t)
	})
}

func TestHandler_ErrorToHTTPResponse(t *testing.T) {
	channelID := uuid.New().String()

	t.Run("ErrSchemaMismatch returns 409", func(t *testing.T) {
		svc := &mockIngestService{}
		h := newTestHandler(svc)
		r := buildRouter(h, testSchema)

		svc.On("Ingest", mock.Anything, mock.Anything).Return(domain.ErrSchemaMismatch)

		body, _ := json.Marshal(map[string]any{"fields": map[string]float64{"temp": 1.0}})
		w := doRequest(r, "POST", "/v1/channels/"+channelID+"/data", body, "application/json")
		assert.Equal(t, http.StatusConflict, w.Code)
	})

	t.Run("ErrDeviceIDMismatch returns 403", func(t *testing.T) {
		svc := &mockIngestService{}
		h := newTestHandler(svc)
		r := buildRouter(h, testSchema)

		svc.On("Ingest", mock.Anything, mock.Anything).Return(domain.ErrDeviceIDMismatch)

		body, _ := json.Marshal(map[string]any{"fields": map[string]float64{"temp": 1.0}})
		w := doRequest(r, "POST", "/v1/channels/"+channelID+"/data", body, "application/json")
		assert.Equal(t, http.StatusForbidden, w.Code)
	})

	t.Run("domain validation error returns 400", func(t *testing.T) {
		svc := &mockIngestService{}
		h := newTestHandler(svc)
		r := buildRouter(h, testSchema)

		svc.On("Ingest", mock.Anything, mock.Anything).Return(domain.ErrEmptyFields)

		body, _ := json.Marshal(map[string]any{"fields": map[string]float64{"temp": 1.0}})
		w := doRequest(r, "POST", "/v1/channels/"+channelID+"/data", body, "application/json")
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})
}
