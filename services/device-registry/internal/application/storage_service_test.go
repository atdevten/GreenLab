package application

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// mockStorageQuerier is an inline mock for StorageQuerier to avoid import cycles.
type mockStorageQuerier struct{ mock.Mock }

func (m *mockStorageQuerier) GetStorageUsage(ctx context.Context) ([]BucketUsage, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]BucketUsage), args.Error(1)
}

func TestStorageService_GetStorageUsage(t *testing.T) {
	ctx := context.Background()

	t.Run("success returns bucket list", func(t *testing.T) {
		querier := new(mockStorageQuerier)
		svc := NewStorageService(querier)

		expected := []BucketUsage{
			{BucketID: "abc", BucketName: "channel-abc", SizeBytes: 0},
			{BucketID: "def", BucketName: "channel-def", SizeBytes: 0},
		}
		querier.On("GetStorageUsage", ctx).Return(expected, nil)

		result, err := svc.GetStorageUsage(ctx)
		require.NoError(t, err)
		assert.Len(t, result, 2)
		assert.Equal(t, expected, result)
		querier.AssertExpectations(t)
	})

	t.Run("empty result", func(t *testing.T) {
		querier := new(mockStorageQuerier)
		svc := NewStorageService(querier)

		querier.On("GetStorageUsage", ctx).Return([]BucketUsage{}, nil)

		result, err := svc.GetStorageUsage(ctx)
		require.NoError(t, err)
		assert.Empty(t, result)
		querier.AssertExpectations(t)
	})

	t.Run("querier error is wrapped and returned", func(t *testing.T) {
		querier := new(mockStorageQuerier)
		svc := NewStorageService(querier)

		querier.On("GetStorageUsage", ctx).Return(nil, errors.New("influx down"))

		result, err := svc.GetStorageUsage(ctx)
		assert.Error(t, err)
		assert.Nil(t, result)
		querier.AssertExpectations(t)
	})
}
