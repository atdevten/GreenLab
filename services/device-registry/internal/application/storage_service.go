package application

import (
	"context"
	"fmt"
)

// BucketUsage represents storage usage for a single channel bucket.
type BucketUsage struct {
	BucketID   string `json:"bucket_id"`
	BucketName string `json:"bucket_name"`
	SizeBytes  int64  `json:"size_bytes"`
}

// StorageQuerier abstracts querying storage usage from InfluxDB.
type StorageQuerier interface {
	GetStorageUsage(ctx context.Context) ([]BucketUsage, error)
}

// StorageService provides storage usage reporting for admin endpoints.
type StorageService struct {
	querier StorageQuerier
}

// NewStorageService creates a new StorageService.
func NewStorageService(querier StorageQuerier) *StorageService {
	return &StorageService{querier: querier}
}

// GetStorageUsage returns storage usage per channel bucket.
func (s *StorageService) GetStorageUsage(ctx context.Context) ([]BucketUsage, error) {
	usage, err := s.querier.GetStorageUsage(ctx)
	if err != nil {
		return nil, fmt.Errorf("StorageService.GetStorageUsage: %w", err)
	}
	return usage, nil
}
