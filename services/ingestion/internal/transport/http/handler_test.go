package http

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
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

const testRequestID = "test-request-id-1234"

func buildRouter(h *Handler, schema domain.DeviceSchema) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("device_schema", schema)
		c.Set("device_id", schema.DeviceID)
		c.Set("request_id", testRequestID)
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

func TestHandler_Ingest_SchemaMismatch(t *testing.T) {
	channelID := uuid.New().String()
	schema := domain.DeviceSchema{
		DeviceID:      "dev-uuid-1",
		ChannelID:     channelID,
		Fields:        []domain.FieldEntry{{Index: 1, Name: "temperature", Type: "float"}},
		SchemaVersion: 5, // server is at version 5
	}

	t.Run("ojson with wrong schema_version returns 409 with current_version and schema_url", func(t *testing.T) {
		svc := &mockIngestService{}
		h := newTestHandler(svc)
		r := buildRouter(h, schema)

		// Device sends sv=3, server has sv=5.
		body := []byte(`{"f":[42.5],"sv":3}`)
		w := doRequest(r, "POST", "/v1/channels/"+channelID+"/data", body, ctOJSON)

		require.Equal(t, http.StatusConflict, w.Code)

		var resp struct {
			Success bool `json:"success"`
			Error   struct {
				Code    string `json:"code"`
				Message string `json:"message"`
				Details struct {
					CurrentVersion uint32 `json:"current_version"`
					SchemaURL      string `json:"schema_url"`
				} `json:"details"`
			} `json:"error"`
		}
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
		assert.False(t, resp.Success)
		assert.Equal(t, "schema_version_mismatch", resp.Error.Code)
		assert.Equal(t, uint32(5), resp.Error.Details.CurrentVersion)
		assert.Equal(t, "/v1/channels/"+channelID+"/schema", resp.Error.Details.SchemaURL)
		svc.AssertNotCalled(t, "IngestBatch")
	})

	t.Run("msgpack with wrong schema_version returns 409 with current_version and schema_url", func(t *testing.T) {
		svc := &mockIngestService{}
		h := newTestHandler(svc)
		r := buildRouter(h, schema)

		// Device sends sv=2, server has sv=5.
		payload := map[string]any{"f": []float64{10.0}, "sv": uint32(2)}
		body, err := msgpack.Marshal(payload)
		require.NoError(t, err)

		w := doRequest(r, "POST", "/v1/channels/"+channelID+"/data", body, ctMsgPack)

		require.Equal(t, http.StatusConflict, w.Code)

		var resp struct {
			Success bool `json:"success"`
			Error   struct {
				Code    string `json:"code"`
				Details struct {
					CurrentVersion uint32 `json:"current_version"`
					SchemaURL      string `json:"schema_url"`
				} `json:"details"`
			} `json:"error"`
		}
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
		assert.False(t, resp.Success)
		assert.Equal(t, "schema_version_mismatch", resp.Error.Code)
		assert.Equal(t, uint32(5), resp.Error.Details.CurrentVersion)
		assert.Equal(t, "/v1/channels/"+channelID+"/schema", resp.Error.Details.SchemaURL)
		svc.AssertNotCalled(t, "IngestBatch")
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

	t.Run("ErrSchemaMismatch sentinel returns 409", func(t *testing.T) {
		svc := &mockIngestService{}
		h := newTestHandler(svc)
		r := buildRouter(h, testSchema)

		svc.On("Ingest", mock.Anything, mock.Anything).Return(domain.ErrSchemaMismatch)

		body, _ := json.Marshal(map[string]any{"fields": map[string]float64{"temp": 1.0}})
		w := doRequest(r, "POST", "/v1/channels/"+channelID+"/data", body, "application/json")
		assert.Equal(t, http.StatusConflict, w.Code)
	})

	t.Run("SchemaMismatchError returns 409 with current_version and schema_url", func(t *testing.T) {
		svc := &mockIngestService{}
		h := newTestHandler(svc)
		r := buildRouter(h, testSchema)

		schemaMismatchErr := &domain.SchemaMismatchError{
			CurrentVersion: 7,
			ChannelID:      channelID,
		}
		svc.On("Ingest", mock.Anything, mock.Anything).Return(schemaMismatchErr)

		body, _ := json.Marshal(map[string]any{"fields": map[string]float64{"temp": 1.0}})
		w := doRequest(r, "POST", "/v1/channels/"+channelID+"/data", body, "application/json")

		require.Equal(t, http.StatusConflict, w.Code)

		var resp struct {
			Success bool `json:"success"`
			Error   struct {
				Code    string `json:"code"`
				Message string `json:"message"`
				Details struct {
					CurrentVersion uint32 `json:"current_version"`
					SchemaURL      string `json:"schema_url"`
				} `json:"details"`
			} `json:"error"`
		}
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
		assert.False(t, resp.Success)
		assert.Equal(t, "schema_version_mismatch", resp.Error.Code)
		assert.Equal(t, uint32(7), resp.Error.Details.CurrentVersion)
		assert.Equal(t, "/v1/channels/"+channelID+"/schema", resp.Error.Details.SchemaURL)
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

// parseIngestResponse decodes the shared response envelope and returns the
// data payload as an IngestResponse.
func parseIngestResponse(t *testing.T, body []byte) IngestResponse {
	t.Helper()
	var envelope struct {
		Data IngestResponse `json:"data"`
	}
	require.NoError(t, json.Unmarshal(body, &envelope))
	return envelope.Data
}

func TestIngestResponse_IncludesChannelIDAndRequestID(t *testing.T) {
	channelID := uuid.New().String()

	t.Run("single JSON ingest — channel_id and request_id present", func(t *testing.T) {
		svc := &mockIngestService{}
		h := newTestHandler(svc)
		r := buildRouter(h, testSchema)

		svc.On("Ingest", mock.Anything, mock.Anything).Return(nil)

		body, _ := json.Marshal(map[string]any{"fields": map[string]float64{"temp": 22.5}})
		w := doRequest(r, "POST", "/v1/channels/"+channelID+"/data", body, "application/json")

		require.Equal(t, http.StatusCreated, w.Code)
		resp := parseIngestResponse(t, w.Body.Bytes())
		assert.Equal(t, 1, resp.Accepted)
		assert.Equal(t, channelID, resp.ChannelID)
		assert.Equal(t, testRequestID, resp.RequestID)
		svc.AssertExpectations(t)
	})

	t.Run("bulk JSON ingest — channel_id and request_id present", func(t *testing.T) {
		svc := &mockIngestService{}
		h := newTestHandler(svc)
		r := buildRouter(h, testSchema)

		svc.On("IngestBatch", mock.Anything, mock.Anything).Return(nil)

		bulkBody, _ := json.Marshal(map[string]any{
			"readings": []map[string]any{
				{"fields": map[string]float64{"temp": 20.0}},
				{"fields": map[string]float64{"temp": 21.0}},
			},
		})
		w := doRequest(r, "POST", "/v1/channels/"+channelID+"/data/bulk", bulkBody, "application/json")

		require.Equal(t, http.StatusCreated, w.Code)
		resp := parseIngestResponse(t, w.Body.Bytes())
		assert.Equal(t, 2, resp.Accepted)
		assert.Equal(t, channelID, resp.ChannelID)
		assert.Equal(t, testRequestID, resp.RequestID)
		svc.AssertExpectations(t)
	})

	t.Run("compact format ingest (ojson) — channel_id and request_id present", func(t *testing.T) {
		schema := domain.DeviceSchema{
			DeviceID:      "dev-uuid-1",
			ChannelID:     channelID,
			Fields:        []domain.FieldEntry{{Index: 1, Name: "temperature", Type: "float"}},
			SchemaVersion: 1,
		}
		svc := &mockIngestService{}
		h := newTestHandler(svc)
		r := buildRouter(h, schema)

		svc.On("IngestBatch", mock.Anything, mock.Anything).Return(nil)

		body := []byte(`{"f":[42.5],"sv":1}`)
		w := doRequest(r, "POST", "/v1/channels/"+channelID+"/data", body, ctOJSON)

		require.Equal(t, http.StatusCreated, w.Code)
		resp := parseIngestResponse(t, w.Body.Bytes())
		assert.Equal(t, 1, resp.Accepted)
		assert.Equal(t, channelID, resp.ChannelID)
		assert.Equal(t, testRequestID, resp.RequestID)
		svc.AssertExpectations(t)
	})
}

// --- ThingSpeak handler tests ---

type mockChannelLookup struct{ mock.Mock }

func (m *mockChannelLookup) Lookup(ctx context.Context, apiKey string) (domain.DeviceSchema, error) {
	args := m.Called(ctx, apiKey)
	return args.Get(0).(domain.DeviceSchema), args.Error(1)
}

func buildThingSpeakRouter(h *Handler, lookup ChannelLookupFunc) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	tsHandler := h.ThingSpeak(lookup, slog.Default(), nil) // nil rdb disables rate limiting in tests
	r.GET("/update", tsHandler)
	r.POST("/update", tsHandler)
	return r
}

func doThingSpeakRequest(r *gin.Engine, query string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, "/update?"+query, nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func TestHandler_ThingSpeak(t *testing.T) {
	schema := domain.DeviceSchema{
		DeviceID:  "dev-uuid-1",
		ChannelID: "chan-uuid-1",
		Fields: []domain.FieldEntry{
			{Index: 1, Name: "temperature", Type: "float"},
			{Index: 2, Name: "humidity", Type: "float"},
		},
		SchemaVersion: 1,
	}

	t.Run("valid field1 and field2 returns Unix timestamp", func(t *testing.T) {
		svc := &mockIngestService{}
		h := newTestHandler(svc)

		lookup := func(_ context.Context, key string) (domain.DeviceSchema, error) {
			if key == "valid-key" {
				return schema, nil
			}
			return domain.DeviceSchema{}, domain.ErrDeviceNotFound
		}

		svc.On("Ingest", mock.Anything, mock.MatchedBy(func(in application.IngestInput) bool {
			return in.ChannelID == "chan-uuid-1" &&
				in.DeviceID == "dev-uuid-1" &&
				in.Fields["temperature"] == 22.5 &&
				in.Fields["humidity"] == 60.0
		})).Return(nil)

		r := buildThingSpeakRouter(h, lookup)
		w := doThingSpeakRequest(r, "api_key=valid-key&field1=22.5&field2=60.0")

		assert.Equal(t, http.StatusOK, w.Code)
		body := w.Body.String()
		assert.NotEqual(t, "0", body)
		// body should be a numeric Unix timestamp
		var ts int64
		_, err := fmt.Sscanf(body, "%d", &ts)
		assert.NoError(t, err)
		assert.Greater(t, ts, int64(0))

		svc.AssertExpectations(t)
	})

	t.Run("missing api_key returns 0", func(t *testing.T) {
		svc := &mockIngestService{}
		h := newTestHandler(svc)
		lookup := func(_ context.Context, _ string) (domain.DeviceSchema, error) {
			return domain.DeviceSchema{}, domain.ErrDeviceNotFound
		}

		r := buildThingSpeakRouter(h, lookup)
		w := doThingSpeakRequest(r, "field1=10.0")

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Equal(t, "0", w.Body.String())
		svc.AssertNotCalled(t, "Ingest")
	})

	t.Run("invalid api_key returns 0", func(t *testing.T) {
		svc := &mockIngestService{}
		h := newTestHandler(svc)
		lookup := func(_ context.Context, _ string) (domain.DeviceSchema, error) {
			return domain.DeviceSchema{}, domain.ErrDeviceNotFound
		}

		r := buildThingSpeakRouter(h, lookup)
		w := doThingSpeakRequest(r, "api_key=bad-key&field1=10.0")

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Equal(t, "0", w.Body.String())
		svc.AssertNotCalled(t, "Ingest")
	})

	t.Run("api_key from X-API-Key header also accepted", func(t *testing.T) {
		svc := &mockIngestService{}
		h := newTestHandler(svc)

		lookup := func(_ context.Context, key string) (domain.DeviceSchema, error) {
			if key == "header-key" {
				return schema, nil
			}
			return domain.DeviceSchema{}, domain.ErrDeviceNotFound
		}

		svc.On("Ingest", mock.Anything, mock.Anything).Return(nil)

		r := buildThingSpeakRouter(h, lookup)
		req := httptest.NewRequest(http.MethodGet, "/update?field1=5.0", nil)
		req.Header.Set("X-API-Key", "header-key")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.NotEqual(t, "0", w.Body.String())
		svc.AssertExpectations(t)
	})

	t.Run("all field params missing returns 0", func(t *testing.T) {
		svc := &mockIngestService{}
		h := newTestHandler(svc)

		lookup := func(_ context.Context, _ string) (domain.DeviceSchema, error) {
			return schema, nil
		}

		r := buildThingSpeakRouter(h, lookup)
		w := doThingSpeakRequest(r, "api_key=valid-key")

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Equal(t, "0", w.Body.String())
		svc.AssertNotCalled(t, "Ingest")
	})

	t.Run("field index not in schema is silently skipped", func(t *testing.T) {
		svc := &mockIngestService{}
		h := newTestHandler(svc)

		// Schema only has field positions 1 and 2; field3 should be skipped.
		lookup := func(_ context.Context, _ string) (domain.DeviceSchema, error) {
			return schema, nil
		}

		svc.On("Ingest", mock.Anything, mock.MatchedBy(func(in application.IngestInput) bool {
			_, has3 := in.Fields["field3"]
			return len(in.Fields) == 1 && in.Fields["temperature"] == 10.0 && !has3
		})).Return(nil)

		r := buildThingSpeakRouter(h, lookup)
		w := doThingSpeakRequest(r, "api_key=valid-key&field1=10.0&field3=99.0")

		assert.Equal(t, http.StatusOK, w.Code)
		assert.NotEqual(t, "0", w.Body.String())
		svc.AssertExpectations(t)
	})

	t.Run("empty field values are skipped", func(t *testing.T) {
		svc := &mockIngestService{}
		h := newTestHandler(svc)

		lookup := func(_ context.Context, _ string) (domain.DeviceSchema, error) {
			return schema, nil
		}

		svc.On("Ingest", mock.Anything, mock.MatchedBy(func(in application.IngestInput) bool {
			return len(in.Fields) == 1 && in.Fields["temperature"] == 15.0
		})).Return(nil)

		r := buildThingSpeakRouter(h, lookup)
		// field2 is empty string, field1 has value
		w := doThingSpeakRequest(r, "api_key=valid-key&field1=15.0&field2=")

		assert.Equal(t, http.StatusOK, w.Code)
		assert.NotEqual(t, "0", w.Body.String())
		svc.AssertExpectations(t)
	})

	t.Run("ingest service failure returns 0", func(t *testing.T) {
		svc := &mockIngestService{}
		h := newTestHandler(svc)

		lookup := func(_ context.Context, _ string) (domain.DeviceSchema, error) {
			return schema, nil
		}

		svc.On("Ingest", mock.Anything, mock.Anything).Return(domain.ErrInvalidChannelID)

		r := buildThingSpeakRouter(h, lookup)
		w := doThingSpeakRequest(r, "api_key=valid-key&field1=5.0")

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Equal(t, "0", w.Body.String())
		svc.AssertExpectations(t)
	})

	t.Run("infrastructure lookup error returns 0", func(t *testing.T) {
		svc := &mockIngestService{}
		h := newTestHandler(svc)

		lookup := func(_ context.Context, _ string) (domain.DeviceSchema, error) {
			return domain.DeviceSchema{}, fmt.Errorf("redis unavailable")
		}

		r := buildThingSpeakRouter(h, lookup)
		w := doThingSpeakRequest(r, "api_key=valid-key&field1=5.0")

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Equal(t, "0", w.Body.String())
		svc.AssertNotCalled(t, "Ingest")
	})

	t.Run("non-numeric field value is silently skipped", func(t *testing.T) {
		svc := &mockIngestService{}
		h := newTestHandler(svc)

		lookup := func(_ context.Context, _ string) (domain.DeviceSchema, error) {
			return schema, nil
		}

		svc.On("Ingest", mock.Anything, mock.MatchedBy(func(in application.IngestInput) bool {
			return len(in.Fields) == 1 && in.Fields["temperature"] == 25.0
		})).Return(nil)

		r := buildThingSpeakRouter(h, lookup)
		// field2 value is not a number, should be skipped
		w := doThingSpeakRequest(r, "api_key=valid-key&field1=25.0&field2=notanumber")

		assert.Equal(t, http.StatusOK, w.Code)
		assert.NotEqual(t, "0", w.Body.String())
		svc.AssertExpectations(t)
	})

	t.Run("POST with form body is accepted", func(t *testing.T) {
		svc := &mockIngestService{}
		h := newTestHandler(svc)

		lookup := func(_ context.Context, key string) (domain.DeviceSchema, error) {
			if key == "valid-key" {
				return schema, nil
			}
			return domain.DeviceSchema{}, domain.ErrDeviceNotFound
		}

		svc.On("Ingest", mock.Anything, mock.MatchedBy(func(in application.IngestInput) bool {
			return in.ChannelID == "chan-uuid-1" && in.Fields["temperature"] == 22.5
		})).Return(nil)

		r := buildThingSpeakRouter(h, lookup)
		body := strings.NewReader("api_key=valid-key&field1=22.5")
		req := httptest.NewRequest(http.MethodPost, "/update", body)
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.NotEqual(t, "0", w.Body.String())
		svc.AssertExpectations(t)
	})
}

func TestBulkIngest_ExceedsMaxReturns400(t *testing.T) {
	svc := &mockIngestService{}
	h := newTestHandler(svc)
	r := buildRouter(h, testSchema)

	readings := make([]map[string]any, maxBulkSize+1)
	for i := range readings {
		readings[i] = map[string]any{"fields": map[string]float64{"temperature": float64(i)}}
	}
	body, _ := json.Marshal(map[string]any{"readings": readings})
	w := doRequest(r, "POST", "/v1/channels/"+testSchema.ChannelID+"/data/bulk", body, "application/json")

	assert.Equal(t, http.StatusBadRequest, w.Code)
	svc.AssertNotCalled(t, "IngestBatch")
}
