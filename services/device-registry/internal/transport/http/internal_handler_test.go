package http

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/greenlab/device-registry/internal/application"
	"github.com/greenlab/device-registry/internal/domain/device"
)

// --- mock internalService ---

type mockInternalService struct{ mock.Mock }

func (m *mockInternalService) ValidateAPIKey(ctx context.Context, apiKey, channelID string) (application.ValidateAPIKeyResult, error) {
	args := m.Called(ctx, apiKey, channelID)
	return args.Get(0).(application.ValidateAPIKeyResult), args.Error(1)
}

func (m *mockInternalService) ResolveChannelByAPIKey(ctx context.Context, apiKey string) (application.ResolveChannelResult, error) {
	args := m.Called(ctx, apiKey)
	return args.Get(0).(application.ResolveChannelResult), args.Error(1)
}

// --- helpers ---

func newInternalRouter(svc internalService) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	h := NewInternalHandler(svc)
	r.POST("/internal/validate-api-key", h.ValidateAPIKey)
	r.GET("/internal/resolve-channel", h.ResolveChannel)
	r.GET("/v1/channels/:id/schema", h.GetChannelSchema)
	return r
}

func postValidateAPIKey(r *gin.Engine, body any) *httptest.ResponseRecorder {
	b, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/internal/validate-api-key", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

// --- tests ---

func TestInternalHandler_ValidateAPIKey(t *testing.T) {
	ctx := context.Background()

	t.Run("valid api_key and channel_id returns 200 with device_id and fields", func(t *testing.T) {
		svc := &mockInternalService{}
		result := application.ValidateAPIKeyResult{
			DeviceID: "dev-uuid-1",
			Fields: []application.FieldEntry{
				{Index: 1, Name: "temperature", Type: "float"},
				{Index: 2, Name: "humidity", Type: "float"},
			},
			SchemaVersion: 1,
		}
		svc.On("ValidateAPIKey", ctx, "key-abc", "chan-123").Return(result, nil)

		w := postValidateAPIKey(newInternalRouter(svc), map[string]string{
			"api_key":    "key-abc",
			"channel_id": "chan-123",
		})

		assert.Equal(t, http.StatusOK, w.Code)

		var resp map[string]any
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))

		data := resp["data"].(map[string]any)
		assert.Equal(t, "dev-uuid-1", data["device_id"])
		assert.Equal(t, float64(1), data["schema_version"])

		fields := data["fields"].([]any)
		require.Len(t, fields, 2)
		field0 := fields[0].(map[string]any)
		assert.Equal(t, float64(1), field0["index"])
		assert.Equal(t, "temperature", field0["name"])
		assert.Equal(t, "float", field0["type"])

		svc.AssertExpectations(t)
	})

	t.Run("unknown api_key returns 401", func(t *testing.T) {
		svc := &mockInternalService{}
		svc.On("ValidateAPIKey", mock.Anything, "bad-key", "chan-123").
			Return(application.ValidateAPIKeyResult{}, device.ErrDeviceNotFound)

		w := postValidateAPIKey(newInternalRouter(svc), map[string]string{
			"api_key":    "bad-key",
			"channel_id": "chan-123",
		})

		assert.Equal(t, http.StatusUnauthorized, w.Code)
		svc.AssertExpectations(t)
	})

	t.Run("wrong channel_id (device does not own channel) returns 401", func(t *testing.T) {
		svc := &mockInternalService{}
		// The service wraps ErrDeviceNotFound when channel doesn't belong to the device.
		notFoundErr := errors.Join(device.ErrDeviceNotFound, errors.New("InternalService.ValidateAPIKey"))
		svc.On("ValidateAPIKey", mock.Anything, "key-abc", "wrong-chan").
			Return(application.ValidateAPIKeyResult{}, notFoundErr)

		w := postValidateAPIKey(newInternalRouter(svc), map[string]string{
			"api_key":    "key-abc",
			"channel_id": "wrong-chan",
		})

		assert.Equal(t, http.StatusUnauthorized, w.Code)
		svc.AssertExpectations(t)
	})

	t.Run("missing api_key in body returns 400", func(t *testing.T) {
		svc := &mockInternalService{}
		w := postValidateAPIKey(newInternalRouter(svc), map[string]string{
			"channel_id": "chan-123",
		})
		assert.Equal(t, http.StatusBadRequest, w.Code)
		svc.AssertNotCalled(t, "ValidateAPIKey")
	})

	t.Run("missing channel_id in body returns 400", func(t *testing.T) {
		svc := &mockInternalService{}
		w := postValidateAPIKey(newInternalRouter(svc), map[string]string{
			"api_key": "key-abc",
		})
		assert.Equal(t, http.StatusBadRequest, w.Code)
		svc.AssertNotCalled(t, "ValidateAPIKey")
	})

	t.Run("empty body returns 400", func(t *testing.T) {
		svc := &mockInternalService{}
		req := httptest.NewRequest(http.MethodPost, "/internal/validate-api-key", bytes.NewReader([]byte{}))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		newInternalRouter(svc).ServeHTTP(w, req)
		assert.Equal(t, http.StatusBadRequest, w.Code)
		svc.AssertNotCalled(t, "ValidateAPIKey")
	})

	t.Run("infrastructure error returns 500", func(t *testing.T) {
		svc := &mockInternalService{}
		svc.On("ValidateAPIKey", mock.Anything, "key-abc", "chan-123").
			Return(application.ValidateAPIKeyResult{}, errors.New("db connection timeout"))

		w := postValidateAPIKey(newInternalRouter(svc), map[string]string{
			"api_key":    "key-abc",
			"channel_id": "chan-123",
		})

		assert.Equal(t, http.StatusInternalServerError, w.Code)
		svc.AssertExpectations(t)
	})
}

func TestInternalHandler_GetChannelSchema(t *testing.T) {
	chanID := "chan-abc"

	getSchema := func(r *gin.Engine, channelID, apiKey string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodGet, "/v1/channels/"+channelID+"/schema", nil)
		if apiKey != "" {
			req.Header.Set("X-API-Key", apiKey)
		}
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		return w
	}

	t.Run("valid API key returns fields and schema_version", func(t *testing.T) {
		svc := &mockInternalService{}
		result := application.ValidateAPIKeyResult{
			DeviceID: "dev-uuid-1",
			Fields: []application.FieldEntry{
				{Index: 1, Name: "temperature", Type: "float"},
				{Index: 2, Name: "humidity", Type: "float"},
			},
			SchemaVersion: 3,
		}
		svc.On("ValidateAPIKey", mock.Anything, "good-key", chanID).Return(result, nil)

		w := getSchema(newInternalRouter(svc), chanID, "good-key")

		assert.Equal(t, http.StatusOK, w.Code)

		var resp map[string]any
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))

		data := resp["data"].(map[string]any)
		assert.Equal(t, float64(3), data["schema_version"])
		fields := data["fields"].([]any)
		require.Len(t, fields, 2)
		assert.Equal(t, "temperature", fields[0].(map[string]any)["name"])
		assert.Equal(t, float64(1), fields[0].(map[string]any)["index"])

		svc.AssertExpectations(t)
	})

	t.Run("missing API key returns 401", func(t *testing.T) {
		svc := &mockInternalService{}
		w := getSchema(newInternalRouter(svc), chanID, "")
		assert.Equal(t, http.StatusUnauthorized, w.Code)
		svc.AssertNotCalled(t, "ValidateAPIKey")
	})

	t.Run("invalid API key returns 401", func(t *testing.T) {
		svc := &mockInternalService{}
		svc.On("ValidateAPIKey", mock.Anything, "bad-key", chanID).
			Return(application.ValidateAPIKeyResult{}, device.ErrDeviceNotFound)

		w := getSchema(newInternalRouter(svc), chanID, "bad-key")
		assert.Equal(t, http.StatusUnauthorized, w.Code)
		svc.AssertExpectations(t)
	})

	t.Run("api_key query param also accepted", func(t *testing.T) {
		svc := &mockInternalService{}
		result := application.ValidateAPIKeyResult{
			DeviceID: "dev-uuid-1",
			Fields:   []application.FieldEntry{{Index: 1, Name: "temp", Type: "float"}},
			SchemaVersion: 1,
		}
		svc.On("ValidateAPIKey", mock.Anything, "query-key", chanID).Return(result, nil)

		req := httptest.NewRequest(http.MethodGet, "/v1/channels/"+chanID+"/schema?api_key=query-key", nil)
		w := httptest.NewRecorder()
		newInternalRouter(svc).ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		svc.AssertExpectations(t)
	})
}

func TestInternalHandler_ResolveChannel(t *testing.T) {
	resolve := func(r *gin.Engine, apiKey string) *httptest.ResponseRecorder {
		url := "/internal/resolve-channel"
		if apiKey != "" {
			url += "?api_key=" + apiKey
		}
		req := httptest.NewRequest(http.MethodGet, url, nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		return w
	}

	t.Run("valid api_key returns device_id, channel_id and fields", func(t *testing.T) {
		svc := &mockInternalService{}
		result := application.ResolveChannelResult{
			ChannelID: "chan-uuid-1",
			ValidateAPIKeyResult: application.ValidateAPIKeyResult{
				DeviceID: "dev-uuid-1",
				Fields: []application.FieldEntry{
					{Index: 1, Name: "temperature", Type: "float"},
					{Index: 2, Name: "humidity", Type: "float"},
				},
				SchemaVersion: 2,
			},
		}
		svc.On("ResolveChannelByAPIKey", mock.Anything, "good-key").Return(result, nil)

		w := resolve(newInternalRouter(svc), "good-key")

		assert.Equal(t, http.StatusOK, w.Code)

		var resp map[string]any
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))

		data := resp["data"].(map[string]any)
		assert.Equal(t, "dev-uuid-1", data["device_id"])
		assert.Equal(t, "chan-uuid-1", data["channel_id"])
		assert.Equal(t, float64(2), data["schema_version"])

		fields := data["fields"].([]any)
		require.Len(t, fields, 2)
		assert.Equal(t, "temperature", fields[0].(map[string]any)["name"])

		svc.AssertExpectations(t)
	})

	t.Run("missing api_key returns 400", func(t *testing.T) {
		svc := &mockInternalService{}
		w := resolve(newInternalRouter(svc), "")
		assert.Equal(t, http.StatusBadRequest, w.Code)
		svc.AssertNotCalled(t, "ResolveChannelByAPIKey")
	})

	t.Run("unknown api_key returns 401", func(t *testing.T) {
		svc := &mockInternalService{}
		svc.On("ResolveChannelByAPIKey", mock.Anything, "bad-key").
			Return(application.ResolveChannelResult{}, device.ErrDeviceNotFound)

		w := resolve(newInternalRouter(svc), "bad-key")
		assert.Equal(t, http.StatusUnauthorized, w.Code)
		svc.AssertExpectations(t)
	})

	t.Run("infrastructure error returns 500", func(t *testing.T) {
		svc := &mockInternalService{}
		svc.On("ResolveChannelByAPIKey", mock.Anything, "key-abc").
			Return(application.ResolveChannelResult{}, errors.New("db timeout"))

		w := resolve(newInternalRouter(svc), "key-abc")
		assert.Equal(t, http.StatusInternalServerError, w.Code)
		svc.AssertExpectations(t)
	})

	t.Run("device with no fields returns empty fields slice", func(t *testing.T) {
		svc := &mockInternalService{}
		result := application.ResolveChannelResult{
			ChannelID: "chan-uuid-2",
			ValidateAPIKeyResult: application.ValidateAPIKeyResult{
				DeviceID:      "dev-uuid-2",
				Fields:        []application.FieldEntry{},
				SchemaVersion: 1,
			},
		}
		svc.On("ResolveChannelByAPIKey", mock.Anything, "key-nofields").Return(result, nil)

		w := resolve(newInternalRouter(svc), "key-nofields")

		assert.Equal(t, http.StatusOK, w.Code)

		var resp map[string]any
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))

		data := resp["data"].(map[string]any)
		assert.Equal(t, "chan-uuid-2", data["channel_id"])
		fields := data["fields"].([]any)
		assert.Empty(t, fields)

		svc.AssertExpectations(t)
	})
}
