package http

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/greenlab/device-registry/internal/application"
	"github.com/greenlab/device-registry/internal/domain/channel"
	"github.com/greenlab/device-registry/internal/domain/device"
	"github.com/greenlab/device-registry/internal/domain/field"
)

// --- mock provisionService ---

type mockProvisionService struct{ mock.Mock }

func (m *mockProvisionService) Provision(ctx context.Context, in application.ProvisionInput) (*application.ProvisionResult, error) {
	args := m.Called(ctx, in)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*application.ProvisionResult), args.Error(1)
}

// --- router helper ---

func newProvisionRouter(svc provisionService) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	h := NewProvisionHandler(svc)
	r.POST("/api/v1/devices/provision", h.Provision)
	return r
}

func postProvision(r *gin.Engine, body any) *httptest.ResponseRecorder {
	b, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/devices/provision", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

// --- fixtures ---

func validProvisionRequest() map[string]any {
	return map[string]any{
		"device": map[string]any{
			"workspace_id": uuid.New().String(),
			"name":         "Sensor Node",
			"description":  "outdoor node",
		},
		"channel": map[string]any{
			"name":        "Main Channel",
			"description": "primary channel",
			"visibility":  "private",
		},
		"fields": []map[string]any{
			{
				"name":       "temperature",
				"label":      "Temperature",
				"unit":       "°C",
				"field_type": "float",
				"position":   1,
			},
		},
	}
}

func makeProvisionResult() *application.ProvisionResult {
	wsID := uuid.New()
	devID := uuid.New()
	chID := uuid.New()
	now := time.Now().UTC()

	d := &device.Device{
		ID:          devID,
		WorkspaceID: wsID,
		Name:        "Sensor Node",
		APIKey:      "ts_testkey123",
		Status:      device.DeviceStatusActive,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	ch := &channel.Channel{
		ID:          chID,
		WorkspaceID: wsID,
		DeviceID:    &devID,
		Name:        "Main Channel",
		Visibility:  channel.ChannelVisibilityPrivate,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	f := &field.Field{
		ID:        uuid.New(),
		ChannelID: chID,
		Name:      "temperature",
		Label:     "Temperature",
		Unit:      "°C",
		FieldType: field.FieldTypeFloat,
		Position:  1,
		CreatedAt: now,
		UpdatedAt: now,
	}
	return &application.ProvisionResult{Device: d, Channel: ch, Fields: []*field.Field{f}}
}

// --- tests ---

func TestProvisionHandler_HappyPath(t *testing.T) {
	svc := &mockProvisionService{}
	result := makeProvisionResult()

	svc.On("Provision", mock.Anything, mock.AnythingOfType("application.ProvisionInput")).
		Return(result, nil)

	w := postProvision(newProvisionRouter(svc), validProvisionRequest())

	assert.Equal(t, http.StatusCreated, w.Code)

	var resp map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))

	data := resp["data"].(map[string]any)
	dev := data["device"].(map[string]any)
	assert.Equal(t, result.Device.ID.String(), dev["id"])
	assert.Equal(t, "ts_testkey123", dev["api_key"])

	ch := data["channel"].(map[string]any)
	assert.Equal(t, result.Channel.ID.String(), ch["id"])
	assert.Equal(t, result.Device.ID.String(), result.Channel.DeviceID.String())

	fields := data["fields"].([]any)
	require.Len(t, fields, 1)
	assert.Equal(t, "temperature", fields[0].(map[string]any)["name"])

	svc.AssertExpectations(t)
}

func TestProvisionHandler_MissingDeviceWorkspaceID(t *testing.T) {
	svc := &mockProvisionService{}
	req := validProvisionRequest()
	delete(req["device"].(map[string]any), "workspace_id")

	w := postProvision(newProvisionRouter(svc), req)
	// validate:"required" failures return 422 Unprocessable Entity
	assert.Equal(t, http.StatusUnprocessableEntity, w.Code)
	svc.AssertNotCalled(t, "Provision")
}

func TestProvisionHandler_MissingDeviceName(t *testing.T) {
	svc := &mockProvisionService{}
	req := validProvisionRequest()
	delete(req["device"].(map[string]any), "name")

	w := postProvision(newProvisionRouter(svc), req)
	assert.Equal(t, http.StatusUnprocessableEntity, w.Code)
	svc.AssertNotCalled(t, "Provision")
}

func TestProvisionHandler_MissingChannelName(t *testing.T) {
	svc := &mockProvisionService{}
	req := validProvisionRequest()
	delete(req["channel"].(map[string]any), "name")

	w := postProvision(newProvisionRouter(svc), req)
	assert.Equal(t, http.StatusUnprocessableEntity, w.Code)
	svc.AssertNotCalled(t, "Provision")
}

func TestProvisionHandler_NeitherChannelNorChannelID(t *testing.T) {
	svc := &mockProvisionService{}
	req := validProvisionRequest()
	delete(req, "channel") // no channel object and no channel_id

	w := postProvision(newProvisionRouter(svc), req)
	assert.Equal(t, http.StatusBadRequest, w.Code)
	svc.AssertNotCalled(t, "Provision")
}

func TestProvisionHandler_BothChannelAndChannelID(t *testing.T) {
	svc := &mockProvisionService{}
	req := validProvisionRequest() // already has "channel"
	req["channel_id"] = uuid.New().String()

	w := postProvision(newProvisionRouter(svc), req)
	assert.Equal(t, http.StatusBadRequest, w.Code)
	svc.AssertNotCalled(t, "Provision")
}

func TestProvisionHandler_ExistingChannelID_HappyPath(t *testing.T) {
	svc := &mockProvisionService{}
	result := makeProvisionResult()

	svc.On("Provision", mock.Anything, mock.AnythingOfType("application.ProvisionInput")).
		Return(result, nil)

	req := map[string]any{
		"device": map[string]any{
			"workspace_id": uuid.New().String(),
			"name":         "Sensor Node",
			"description":  "outdoor node",
		},
		"channel_id": result.Channel.ID.String(),
		"fields": []map[string]any{
			{"name": "temperature", "label": "Temperature", "unit": "°C", "field_type": "float", "position": 1},
		},
	}

	w := postProvision(newProvisionRouter(svc), req)
	assert.Equal(t, http.StatusCreated, w.Code)

	var resp map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	data := resp["data"].(map[string]any)
	assert.Equal(t, result.Channel.ID.String(), data["channel"].(map[string]any)["id"])

	// Verify ExistingChannelID was forwarded to the service.
	capturedIn := svc.Calls[0].Arguments.Get(1).(application.ProvisionInput)
	assert.Equal(t, result.Channel.ID.String(), capturedIn.ExistingChannelID)
	assert.Empty(t, capturedIn.Channel.Name)

	svc.AssertExpectations(t)
}

func TestProvisionHandler_ExistingChannel_NotFound(t *testing.T) {
	svc := &mockProvisionService{}
	svc.On("Provision", mock.Anything, mock.AnythingOfType("application.ProvisionInput")).
		Return(nil, channel.ErrChannelNotFound)

	req := map[string]any{
		"device":     map[string]any{"workspace_id": uuid.New().String(), "name": "Node"},
		"channel_id": uuid.New().String(),
	}

	w := postProvision(newProvisionRouter(svc), req)
	assert.Equal(t, http.StatusNotFound, w.Code)
	svc.AssertExpectations(t)
}

func TestProvisionHandler_ServiceReturnsDeviceValidationError(t *testing.T) {
	svc := &mockProvisionService{}
	svc.On("Provision", mock.Anything, mock.AnythingOfType("application.ProvisionInput")).
		Return(nil, device.ErrInvalidName)

	w := postProvision(newProvisionRouter(svc), validProvisionRequest())
	assert.Equal(t, http.StatusBadRequest, w.Code)
	svc.AssertExpectations(t)
}

func TestProvisionHandler_ServiceReturnsChannelValidationError(t *testing.T) {
	svc := &mockProvisionService{}
	svc.On("Provision", mock.Anything, mock.AnythingOfType("application.ProvisionInput")).
		Return(nil, channel.ErrInvalidVisibility)

	w := postProvision(newProvisionRouter(svc), validProvisionRequest())
	assert.Equal(t, http.StatusBadRequest, w.Code)
	svc.AssertExpectations(t)
}

func TestProvisionHandler_ServiceReturnsFieldValidationError(t *testing.T) {
	svc := &mockProvisionService{}
	svc.On("Provision", mock.Anything, mock.AnythingOfType("application.ProvisionInput")).
		Return(nil, field.ErrInvalidPosition)

	w := postProvision(newProvisionRouter(svc), validProvisionRequest())
	assert.Equal(t, http.StatusBadRequest, w.Code)
	svc.AssertExpectations(t)
}

func TestProvisionHandler_ServiceReturnsInfraError(t *testing.T) {
	svc := &mockProvisionService{}
	svc.On("Provision", mock.Anything, mock.AnythingOfType("application.ProvisionInput")).
		Return(nil, errors.New("tx commit failed: db connection lost"))

	w := postProvision(newProvisionRouter(svc), validProvisionRequest())
	assert.Equal(t, http.StatusInternalServerError, w.Code)
	svc.AssertExpectations(t)
}

func TestProvisionHandler_InvalidJSON(t *testing.T) {
	svc := &mockProvisionService{}
	req := httptest.NewRequest(http.MethodPost, "/api/v1/devices/provision", bytes.NewReader([]byte("not json")))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	newProvisionRouter(svc).ServeHTTP(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)
	svc.AssertNotCalled(t, "Provision")
}

func TestMapProvisionError(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		wantCode int
	}{
		{"device invalid name", device.ErrInvalidName, http.StatusBadRequest},
		{"device invalid status", device.ErrInvalidStatus, http.StatusBadRequest},
		{"channel invalid name", channel.ErrInvalidName, http.StatusBadRequest},
		{"channel invalid visibility", channel.ErrInvalidVisibility, http.StatusBadRequest},
		{"channel not found", channel.ErrChannelNotFound, http.StatusBadRequest},
		{"field invalid name", field.ErrInvalidName, http.StatusBadRequest},
		{"field invalid position", field.ErrInvalidPosition, http.StatusBadRequest},
		{"field invalid type", field.ErrInvalidFieldType, http.StatusBadRequest},
		{"unknown error", errors.New("db timeout"), http.StatusInternalServerError},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			apiErr := mapProvisionError(tc.err)
			// Verify the HTTP status is reflected in the apierr struct.
			type statusCoder interface{ StatusCode() int }
			if sc, ok := apiErr.(statusCoder); ok {
				assert.Equal(t, tc.wantCode, sc.StatusCode())
			}
		})
	}
}
