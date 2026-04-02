package http

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/greenlab/alert-notification/internal/application"
	"github.com/greenlab/alert-notification/internal/domain/alert"
	"github.com/greenlab/alert-notification/internal/domain/delivery"
)

func init() {
	gin.SetMode(gin.TestMode)
}

// --- mock alertService ---

type mockAlertService struct{ mock.Mock }

func (m *mockAlertService) CreateRule(ctx context.Context, in application.CreateRuleInput) (*alert.Rule, error) {
	args := m.Called(ctx, in)
	r, _ := args.Get(0).(*alert.Rule)
	return r, args.Error(1)
}
func (m *mockAlertService) GetRule(ctx context.Context, id string) (*alert.Rule, error) {
	args := m.Called(ctx, id)
	r, _ := args.Get(0).(*alert.Rule)
	return r, args.Error(1)
}
func (m *mockAlertService) ListRules(ctx context.Context, workspaceID string, limit, offset int) ([]*alert.Rule, int64, error) {
	args := m.Called(ctx, workspaceID, limit, offset)
	r, _ := args.Get(0).([]*alert.Rule)
	return r, args.Get(1).(int64), args.Error(2)
}
func (m *mockAlertService) UpdateRule(ctx context.Context, id string, in application.UpdateRuleInput) (*alert.Rule, error) {
	args := m.Called(ctx, id, in)
	r, _ := args.Get(0).(*alert.Rule)
	return r, args.Error(1)
}
func (m *mockAlertService) DeleteRule(ctx context.Context, id string) error {
	return m.Called(ctx, id).Error(0)
}
func (m *mockAlertService) ListDeliveries(ctx context.Context, ruleID string, limit, offset int) ([]*delivery.Log, int64, error) {
	args := m.Called(ctx, ruleID, limit, offset)
	logs, _ := args.Get(0).([]*delivery.Log)
	return logs, args.Get(1).(int64), args.Error(2)
}
func (m *mockAlertService) VerifyWebhookSignature(ctx context.Context, id, payload, signature string) (bool, error) {
	args := m.Called(ctx, id, payload, signature)
	return args.Bool(0), args.Error(1)
}

// --- helpers ---

func newTestHandler(t *testing.T) (*AlertHandler, *mockAlertService) {
	t.Helper()
	svc := &mockAlertService{}
	h := NewAlertHandler(svc, slog.Default())
	return h, svc
}

func setupRouter(h *AlertHandler) *gin.Engine {
	r := gin.New()
	r.POST("/alert-rules", h.CreateRule)
	r.GET("/alert-rules/:id", h.GetRule)
	r.PUT("/alert-rules/:id", h.UpdateRule)
	r.DELETE("/alert-rules/:id", h.DeleteRule)
	r.POST("/alert-rules/:id/verify-signature", h.VerifySignature)
	r.GET("/health", h.Health)
	return r
}

func jsonBody(t *testing.T, v interface{}) *bytes.Buffer {
	t.Helper()
	b, err := json.Marshal(v)
	require.NoError(t, err)
	return bytes.NewBuffer(b)
}

// --- tests ---

func TestHealth(t *testing.T) {
	h, _ := newTestHandler(t)
	r := setupRouter(h)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestCreateRule_Success(t *testing.T) {
	h, svc := newTestHandler(t)
	r := setupRouter(h)

	threshold := 80.0
	ruleID := uuid.New()
	chID := uuid.New()
	wsID := uuid.New()

	svc.On("CreateRule", mock.Anything, mock.MatchedBy(func(in application.CreateRuleInput) bool {
		return in.Name == "High Temp" && in.WebhookSecret == "my-secret"
	})).Return(&alert.Rule{
		ID:          ruleID,
		ChannelID:   chID,
		WorkspaceID: wsID,
		Name:        "High Temp",
		Threshold:   80.0,
		Severity:    alert.SeverityWarning,
		Enabled:     true,
	}, nil)

	body := jsonBody(t, map[string]interface{}{
		"channel_id":   chID.String(),
		"workspace_id": wsID.String(),
		"name":         "High Temp",
		"field_name":   "temperature",
		"condition":    "gt",
		"threshold":    threshold,
		"severity":     "warning",
		"secret":       "my-secret",
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/alert-rules", body)
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)
	svc.AssertExpectations(t)
}

func TestCreateRule_MissingRequiredFields_Returns422(t *testing.T) {
	h, _ := newTestHandler(t)
	r := setupRouter(h)

	body := jsonBody(t, map[string]interface{}{
		"name": "Missing fields",
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/alert-rules", body)
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	// Validation errors (required fields missing) return 422 Unprocessable Entity.
	assert.Equal(t, http.StatusUnprocessableEntity, w.Code)
}

func TestCreateRule_ServiceError_Returns500(t *testing.T) {
	h, svc := newTestHandler(t)
	r := setupRouter(h)

	chID := uuid.New()
	wsID := uuid.New()
	threshold := 80.0

	svc.On("CreateRule", mock.Anything, mock.Anything).Return(nil, errors.New("db error"))

	body := jsonBody(t, map[string]interface{}{
		"channel_id":   chID.String(),
		"workspace_id": wsID.String(),
		"name":         "r",
		"field_name":   "f",
		"condition":    "gt",
		"threshold":    threshold,
		"severity":     "info",
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/alert-rules", body)
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestGetRule_Success(t *testing.T) {
	h, svc := newTestHandler(t)
	r := setupRouter(h)

	id := uuid.New()
	svc.On("GetRule", mock.Anything, id.String()).Return(&alert.Rule{
		ID:      id,
		Name:    "test rule",
		Enabled: true,
	}, nil)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/alert-rules/"+id.String(), nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	svc.AssertExpectations(t)
}

func TestGetRule_NotFound_Returns404(t *testing.T) {
	h, svc := newTestHandler(t)
	r := setupRouter(h)

	id := uuid.New()
	svc.On("GetRule", mock.Anything, id.String()).Return(nil, alert.ErrRuleNotFound)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/alert-rules/"+id.String(), nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
	svc.AssertExpectations(t)
}

func TestDeleteRule_Success(t *testing.T) {
	h, svc := newTestHandler(t)
	r := setupRouter(h)

	id := uuid.New()
	svc.On("DeleteRule", mock.Anything, id.String()).Return(nil)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodDelete, "/alert-rules/"+id.String(), nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNoContent, w.Code)
	svc.AssertExpectations(t)
}

func TestDeleteRule_NotFound_Returns404(t *testing.T) {
	h, svc := newTestHandler(t)
	r := setupRouter(h)

	id := uuid.New()
	svc.On("DeleteRule", mock.Anything, id.String()).Return(alert.ErrRuleNotFound)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodDelete, "/alert-rules/"+id.String(), nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestVerifySignature_ValidSignature_Returns200(t *testing.T) {
	h, svc := newTestHandler(t)
	r := setupRouter(h)

	id := uuid.New()
	payload := `{"sensor":"temp","value":42}`
	sig := "sha256=" + alert.ComputeHMAC("my-secret", payload)

	svc.On("VerifyWebhookSignature", mock.Anything, id.String(), payload, sig).
		Return(true, nil)

	body := jsonBody(t, map[string]string{
		"payload":   payload,
		"signature": sig,
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/alert-rules/"+id.String()+"/verify-signature", body)
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	var resp map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	// Response is wrapped in data field by response.OK
	data, ok := resp["data"].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, true, data["valid"])
	svc.AssertExpectations(t)
}

func TestVerifySignature_InvalidSignature_Returns200WithFalse(t *testing.T) {
	h, svc := newTestHandler(t)
	r := setupRouter(h)

	id := uuid.New()
	svc.On("VerifyWebhookSignature", mock.Anything, id.String(), "payload", "sha256=wrong").
		Return(false, nil)

	body := jsonBody(t, map[string]string{
		"payload":   "payload",
		"signature": "sha256=wrong",
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/alert-rules/"+id.String()+"/verify-signature", body)
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	svc.AssertExpectations(t)
}

func TestVerifySignature_MissingFields_Returns422(t *testing.T) {
	h, _ := newTestHandler(t)
	r := setupRouter(h)

	id := uuid.New()
	body := jsonBody(t, map[string]string{"payload": "something"}) // missing signature

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/alert-rules/"+id.String()+"/verify-signature", body)
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	// Validation errors (required fields missing) return 422 Unprocessable Entity.
	assert.Equal(t, http.StatusUnprocessableEntity, w.Code)
}

func TestVerifySignature_RuleNotFound_Returns404(t *testing.T) {
	h, svc := newTestHandler(t)
	r := setupRouter(h)

	id := uuid.New()
	svc.On("VerifyWebhookSignature", mock.Anything, id.String(), "p", "s").
		Return(false, alert.ErrRuleNotFound)

	body := jsonBody(t, map[string]string{"payload": "p", "signature": "s"})
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/alert-rules/"+id.String()+"/verify-signature", body)
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestVerifySignature_NoSecret_Returns400(t *testing.T) {
	h, svc := newTestHandler(t)
	r := setupRouter(h)

	id := uuid.New()
	svc.On("VerifyWebhookSignature", mock.Anything, id.String(), "p", "s").
		Return(false, alert.ErrNoWebhookSecret)

	body := jsonBody(t, map[string]string{"payload": "p", "signature": "s"})
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/alert-rules/"+id.String()+"/verify-signature", body)
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestVerifySignature_InvalidRuleID_Returns400(t *testing.T) {
	h, svc := newTestHandler(t)
	r := setupRouter(h)

	id := uuid.New()
	svc.On("VerifyWebhookSignature", mock.Anything, id.String(), "p", "s").
		Return(false, alert.ErrInvalidRuleID)

	body := jsonBody(t, map[string]string{"payload": "p", "signature": "s"})
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/alert-rules/"+id.String()+"/verify-signature", body)
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}
