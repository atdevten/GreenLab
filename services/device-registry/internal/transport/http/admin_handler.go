package http

import (
	"context"

	"github.com/gin-gonic/gin"
	"github.com/greenlab/device-registry/internal/application"
	"github.com/greenlab/shared/pkg/apierr"
	"github.com/greenlab/shared/pkg/response"
)

// storageService is the interface the admin handler depends on.
type storageService interface {
	GetStorageUsage(ctx context.Context) ([]application.BucketUsage, error)
}

// AdminHandler serves admin-only endpoints (requires JWT auth with admin role).
type AdminHandler struct {
	storageSvc storageService
}

// NewAdminHandler creates a new AdminHandler.
func NewAdminHandler(storageSvc storageService) *AdminHandler {
	return &AdminHandler{storageSvc: storageSvc}
}

// storageUsageItem is a single bucket entry in the usage response.
type storageUsageItem struct {
	BucketID   string `json:"bucket_id"`
	BucketName string `json:"bucket_name"`
	SizeBytes  int64  `json:"size_bytes"`
}

// storageUsageResponse is the response body for GET /api/v1/admin/storage/usage.
type storageUsageResponse struct {
	Buckets []storageUsageItem `json:"buckets"`
	Total   int                `json:"total"`
}

// GetStorageUsage godoc
// @Summary      Get InfluxDB storage usage per channel bucket
// @Tags         admin
// @Produce      json
// @Success      200  {object}  storageUsageResponse
// @Failure      500  {object}  map[string]interface{}
// @Security     BearerAuth
// @Router       /api/v1/admin/storage/usage [get]
func (h *AdminHandler) GetStorageUsage(c *gin.Context) {
	usage, err := h.storageSvc.GetStorageUsage(c.Request.Context())
	if err != nil {
		response.Error(c, apierr.Internal(err))
		return
	}

	items := make([]storageUsageItem, len(usage))
	for i, u := range usage {
		items[i] = storageUsageItem{
			BucketID:   u.BucketID,
			BucketName: u.BucketName,
			SizeBytes:  u.SizeBytes,
		}
	}

	response.OK(c, storageUsageResponse{
		Buckets: items,
		Total:   len(items),
	})
}
