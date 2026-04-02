package application

import (
	"context"
	"errors"
	"log/slog"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/greenlab/device-registry/internal/domain/channel"
	mockapplication "github.com/greenlab/device-registry/internal/mocks/application"
	mockchannel "github.com/greenlab/device-registry/internal/mocks/channel"
)

func newTestChannelService(t *testing.T) (*ChannelService, *mockchannel.MockChannelRepository, *mockapplication.MockRetentionManager) {
	t.Helper()
	repo := mockchannel.NewMockChannelRepository(t)
	retention := mockapplication.NewMockRetentionManager(t)
	svc := NewChannelService(repo, retention, slog.Default())
	return svc, repo, retention
}

func TestCreateChannel(t *testing.T) {
	ctx := context.Background()

	t.Run("success with default retention", func(t *testing.T) {
		svc, repo, retention := newTestChannelService(t)
		wsID := uuid.New()
		repo.On("Create", ctx, mock.AnythingOfType("*channel.Channel")).Return(nil)
		retention.On("SetRetention", ctx, mock.AnythingOfType("string"), channel.DefaultRetentionDays).Return(nil)

		ch, err := svc.CreateChannel(ctx, CreateChannelInput{
			WorkspaceID: wsID.String(),
			Name:        "My Channel",
			Visibility:  "public",
		})
		require.NoError(t, err)
		assert.Equal(t, "My Channel", ch.Name)
		assert.Equal(t, channel.ChannelVisibilityPublic, ch.Visibility)
		assert.Equal(t, channel.DefaultRetentionDays, ch.RetentionDays)
		repo.AssertExpectations(t)
		retention.AssertExpectations(t)
	})

	t.Run("success with custom retention", func(t *testing.T) {
		svc, repo, retention := newTestChannelService(t)
		wsID := uuid.New()
		repo.On("Create", ctx, mock.AnythingOfType("*channel.Channel")).Return(nil)
		retention.On("SetRetention", ctx, mock.AnythingOfType("string"), 30).Return(nil)

		ch, err := svc.CreateChannel(ctx, CreateChannelInput{
			WorkspaceID:   wsID.String(),
			Name:          "My Channel",
			RetentionDays: 30,
		})
		require.NoError(t, err)
		assert.Equal(t, 30, ch.RetentionDays)
		repo.AssertExpectations(t)
		retention.AssertExpectations(t)
	})

	t.Run("invalid retention_days returns domain error", func(t *testing.T) {
		svc, _, _ := newTestChannelService(t)
		ch, err := svc.CreateChannel(ctx, CreateChannelInput{
			WorkspaceID:   uuid.New().String(),
			Name:          "My Channel",
			RetentionDays: 400,
		})
		assert.ErrorIs(t, err, channel.ErrInvalidRetention)
		assert.Nil(t, ch)
	})

	t.Run("retention manager error is logged not returned", func(t *testing.T) {
		svc, repo, retention := newTestChannelService(t)
		wsID := uuid.New()
		repo.On("Create", ctx, mock.AnythingOfType("*channel.Channel")).Return(nil)
		retention.On("SetRetention", ctx, mock.AnythingOfType("string"), channel.DefaultRetentionDays).
			Return(errors.New("influx unavailable"))

		// Should succeed even though retention fails
		ch, err := svc.CreateChannel(ctx, CreateChannelInput{
			WorkspaceID: wsID.String(),
			Name:        "My Channel",
		})
		require.NoError(t, err)
		assert.NotNil(t, ch)
		repo.AssertExpectations(t)
		retention.AssertExpectations(t)
	})

	t.Run("empty name returns domain error", func(t *testing.T) {
		svc, _, _ := newTestChannelService(t)
		ch, err := svc.CreateChannel(ctx, CreateChannelInput{WorkspaceID: uuid.New().String(), Name: ""})
		assert.ErrorIs(t, err, channel.ErrInvalidName)
		assert.Nil(t, ch)
	})

	t.Run("empty visibility defaults to private", func(t *testing.T) {
		svc, repo, retention := newTestChannelService(t)
		repo.On("Create", ctx, mock.AnythingOfType("*channel.Channel")).Return(nil)
		retention.On("SetRetention", ctx, mock.AnythingOfType("string"), channel.DefaultRetentionDays).Return(nil)

		ch, err := svc.CreateChannel(ctx, CreateChannelInput{WorkspaceID: uuid.New().String(), Name: "My Channel"})
		require.NoError(t, err)
		assert.Equal(t, channel.ChannelVisibilityPrivate, ch.Visibility)
		repo.AssertExpectations(t)
		retention.AssertExpectations(t)
	})

	t.Run("invalid workspace_id", func(t *testing.T) {
		svc, _, _ := newTestChannelService(t)
		ch, err := svc.CreateChannel(ctx, CreateChannelInput{WorkspaceID: "not-a-uuid", Name: "My Channel"})
		assert.Error(t, err)
		assert.Nil(t, ch)
	})
}

func TestGetChannel(t *testing.T) {
	ctx := context.Background()

	t.Run("success", func(t *testing.T) {
		svc, repo, _ := newTestChannelService(t)
		id := uuid.New()
		expected := &channel.Channel{ID: id, Name: "My Channel"}
		repo.On("GetByID", ctx, id).Return(expected, nil)

		ch, err := svc.GetChannel(ctx, id.String())
		require.NoError(t, err)
		assert.Equal(t, expected, ch)
		repo.AssertExpectations(t)
	})

	t.Run("not found", func(t *testing.T) {
		svc, repo, _ := newTestChannelService(t)
		id := uuid.New()
		repo.On("GetByID", ctx, id).Return(nil, channel.ErrChannelNotFound)

		ch, err := svc.GetChannel(ctx, id.String())
		assert.ErrorIs(t, err, channel.ErrChannelNotFound)
		assert.Nil(t, ch)
		repo.AssertExpectations(t)
	})
}

func TestListChannels(t *testing.T) {
	ctx := context.Background()

	t.Run("success", func(t *testing.T) {
		svc, repo, _ := newTestChannelService(t)
		wsID := uuid.New()
		expected := []*channel.Channel{{ID: uuid.New(), Name: "ch1"}}
		repo.On("ListByWorkspace", ctx, wsID, 10, 0).Return(expected, int64(1), nil)

		channels, total, err := svc.ListChannels(ctx, wsID.String(), 10, 0)
		require.NoError(t, err)
		assert.Equal(t, int64(1), total)
		assert.Len(t, channels, 1)
		repo.AssertExpectations(t)
	})

	t.Run("invalid workspace_id", func(t *testing.T) {
		svc, _, _ := newTestChannelService(t)
		channels, total, err := svc.ListChannels(ctx, "not-a-uuid", 10, 0)
		assert.Error(t, err)
		assert.Nil(t, channels)
		assert.Equal(t, int64(0), total)
	})
}

func TestUpdateChannel(t *testing.T) {
	ctx := context.Background()

	t.Run("success", func(t *testing.T) {
		svc, repo, retention := newTestChannelService(t)
		id := uuid.New()
		existing := &channel.Channel{ID: id, Name: "Old", Visibility: channel.ChannelVisibilityPrivate, RetentionDays: 90}
		repo.On("GetByID", ctx, id).Return(existing, nil)
		repo.On("Update", ctx, mock.AnythingOfType("*channel.Channel")).Return(nil)
		retention.On("SetRetention", ctx, id.String(), 90).Return(nil)

		ch, err := svc.UpdateChannel(ctx, id.String(), UpdateChannelInput{Name: "New"})
		require.NoError(t, err)
		assert.Equal(t, "New", ch.Name)
		repo.AssertExpectations(t)
		retention.AssertExpectations(t)
	})

	t.Run("update retention_days", func(t *testing.T) {
		svc, repo, retention := newTestChannelService(t)
		id := uuid.New()
		existing := &channel.Channel{ID: id, Name: "Old", Visibility: channel.ChannelVisibilityPrivate, RetentionDays: 90}
		repo.On("GetByID", ctx, id).Return(existing, nil)
		repo.On("Update", ctx, mock.AnythingOfType("*channel.Channel")).Return(nil)
		retention.On("SetRetention", ctx, id.String(), 180).Return(nil)

		ch, err := svc.UpdateChannel(ctx, id.String(), UpdateChannelInput{RetentionDays: 180})
		require.NoError(t, err)
		assert.Equal(t, 180, ch.RetentionDays)
		repo.AssertExpectations(t)
		retention.AssertExpectations(t)
	})

	t.Run("invalid retention_days on update", func(t *testing.T) {
		svc, repo, _ := newTestChannelService(t)
		id := uuid.New()
		existing := &channel.Channel{ID: id, Name: "Old", Visibility: channel.ChannelVisibilityPrivate, RetentionDays: 90}
		repo.On("GetByID", ctx, id).Return(existing, nil)

		ch, err := svc.UpdateChannel(ctx, id.String(), UpdateChannelInput{RetentionDays: 400})
		assert.ErrorIs(t, err, channel.ErrInvalidRetention)
		assert.Nil(t, ch)
		repo.AssertExpectations(t)
	})

	t.Run("repo error on get", func(t *testing.T) {
		svc, repo, _ := newTestChannelService(t)
		id := uuid.New()
		repo.On("GetByID", ctx, id).Return(nil, errors.New("db error"))

		ch, err := svc.UpdateChannel(ctx, id.String(), UpdateChannelInput{Name: "New"})
		assert.Error(t, err)
		assert.Nil(t, ch)
		repo.AssertExpectations(t)
	})
}

func TestDeleteChannel(t *testing.T) {
	ctx := context.Background()

	t.Run("success", func(t *testing.T) {
		svc, repo, _ := newTestChannelService(t)
		id := uuid.New()
		repo.On("Delete", ctx, id).Return(nil)

		err := svc.DeleteChannel(ctx, id.String())
		require.NoError(t, err)
		repo.AssertExpectations(t)
	})

	t.Run("invalid id", func(t *testing.T) {
		svc, _, _ := newTestChannelService(t)
		err := svc.DeleteChannel(ctx, "bad-uuid")
		assert.Error(t, err)
	})
}
