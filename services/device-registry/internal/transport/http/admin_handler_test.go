package http

import (
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
)

// --- mock storageService ---

type mockStorageService struct{ mock.Mock }

func (m *mockStorageService) GetStorageUsage(ctx context.Context) ([]application.BucketUsage, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]application.BucketUsage), args.Error(1)
}

// --- helpers ---

func newAdminRouter(svc storageService) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	h := NewAdminHandler(svc)
	r.GET("/api/v1/admin/storage/usage", h.GetStorageUsage)
	return r
}

func getStorageUsage(r *gin.Engine) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/storage/usage", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

// --- tests ---

func TestAdminHandler_GetStorageUsage(t *testing.T) {
	t.Run("success returns bucket list", func(t *testing.T) {
		svc := new(mockStorageService)
		svc.On("GetStorageUsage", mock.Anything).Return([]application.BucketUsage{
			{BucketID: "abc123", BucketName: "channel-abc123", SizeBytes: 0},
			{BucketID: "def456", BucketName: "channel-def456", SizeBytes: 0},
		}, nil)

		r := newAdminRouter(svc)
		w := getStorageUsage(r)

		require.Equal(t, http.StatusOK, w.Code)
		var body map[string]interface{}
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))

		data, ok := body["data"].(map[string]interface{})
		require.True(t, ok)
		buckets, ok := data["buckets"].([]interface{})
		require.True(t, ok)
		assert.Len(t, buckets, 2)
		assert.Equal(t, float64(2), data["total"])

		svc.AssertExpectations(t)
	})

	t.Run("empty bucket list", func(t *testing.T) {
		svc := new(mockStorageService)
		svc.On("GetStorageUsage", mock.Anything).Return([]application.BucketUsage{}, nil)

		r := newAdminRouter(svc)
		w := getStorageUsage(r)

		require.Equal(t, http.StatusOK, w.Code)
		var body map[string]interface{}
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))

		data := body["data"].(map[string]interface{})
		buckets := data["buckets"].([]interface{})
		assert.Len(t, buckets, 0)
		assert.Equal(t, float64(0), data["total"])

		svc.AssertExpectations(t)
	})

	t.Run("service error returns 500", func(t *testing.T) {
		svc := new(mockStorageService)
		svc.On("GetStorageUsage", mock.Anything).Return(nil, errors.New("influx down"))

		r := newAdminRouter(svc)
		w := getStorageUsage(r)

		assert.Equal(t, http.StatusInternalServerError, w.Code)
		svc.AssertExpectations(t)
	})
}
