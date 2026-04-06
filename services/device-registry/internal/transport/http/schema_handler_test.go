package http

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/greenlab/device-registry/internal/domain/channel"
)

// --- mocks ---

type mockSchemaChannelGetter struct{ mock.Mock }

func (m *mockSchemaChannelGetter) GetChannel(ctx context.Context, id string) (*channel.Channel, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*channel.Channel), args.Error(1)
}

type mockSchemaDeprecationStore struct{ mock.Mock }

func (m *mockSchemaDeprecationStore) SetForceDeprecated(ctx context.Context, channelID string) error {
	return m.Called(ctx, channelID).Error(0)
}

// --- helpers ---

func newSchemaRouter(channelSvc schemaChannelGetter, deprecStore schemaDeprecationStore) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	h := NewSchemaHandler(channelSvc, deprecStore)
	r.POST("/api/v1/channels/:id/schema/force-deprecate", h.ForceDeprecateSchema)
	return r
}

func doForceDeprecate(r *gin.Engine, channelID string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, "/api/v1/channels/"+channelID+"/schema/force-deprecate", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

// --- tests ---

func TestSchemaHandler_ForceDeprecateSchema_Success(t *testing.T) {
	channelSvc := new(mockSchemaChannelGetter)
	deprecStore := new(mockSchemaDeprecationStore)

	ch := &channel.Channel{}
	channelSvc.On("GetChannel", mock.Anything, "chan-123").Return(ch, nil)
	deprecStore.On("SetForceDeprecated", mock.Anything, "chan-123").Return(nil)

	r := newSchemaRouter(channelSvc, deprecStore)
	w := doForceDeprecate(r, "chan-123")

	require.Equal(t, http.StatusOK, w.Code)

	var body struct {
		Data struct {
			ChannelID  string    `json:"channel_id"`
			Deprecated bool      `json:"deprecated"`
			ExpiresAt  time.Time `json:"expires_at"`
			Note       string    `json:"note"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.Equal(t, "chan-123", body.Data.ChannelID)
	assert.True(t, body.Data.Deprecated)
	assert.WithinDuration(t, time.Now().Add(48*time.Hour), body.Data.ExpiresAt, 5*time.Second)
	assert.NotEmpty(t, body.Data.Note)

	channelSvc.AssertExpectations(t)
	deprecStore.AssertExpectations(t)
}

func TestSchemaHandler_ForceDeprecateSchema_ChannelNotFound(t *testing.T) {
	channelSvc := new(mockSchemaChannelGetter)
	deprecStore := new(mockSchemaDeprecationStore)

	channelSvc.On("GetChannel", mock.Anything, "missing").Return(nil, channel.ErrChannelNotFound)

	r := newSchemaRouter(channelSvc, deprecStore)
	w := doForceDeprecate(r, "missing")

	assert.Equal(t, http.StatusNotFound, w.Code)
	deprecStore.AssertNotCalled(t, "SetForceDeprecated")
	channelSvc.AssertExpectations(t)
}

func TestSchemaHandler_ForceDeprecateSchema_StoreError(t *testing.T) {
	channelSvc := new(mockSchemaChannelGetter)
	deprecStore := new(mockSchemaDeprecationStore)

	ch := &channel.Channel{}
	channelSvc.On("GetChannel", mock.Anything, "chan-123").Return(ch, nil)
	deprecStore.On("SetForceDeprecated", mock.Anything, "chan-123").Return(errors.New("redis unavailable"))

	r := newSchemaRouter(channelSvc, deprecStore)
	w := doForceDeprecate(r, "chan-123")

	assert.Equal(t, http.StatusInternalServerError, w.Code)
	channelSvc.AssertExpectations(t)
	deprecStore.AssertExpectations(t)
}
