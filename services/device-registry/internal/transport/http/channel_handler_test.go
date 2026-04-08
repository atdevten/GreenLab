package http

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/greenlab/device-registry/internal/domain/channel"
	"github.com/stretchr/testify/assert"
)

func TestToChannelResponse_Tags(t *testing.T) {
	tests := []struct {
		name         string
		tagsJSON     []byte
		expectedTags []string
	}{
		{
			name:         "populated tags are deserialized",
			tagsJSON:     []byte(`["env:prod","region:us"]`),
			expectedTags: []string{"env:prod", "region:us"},
		},
		{
			name:         "empty JSON array returns empty slice",
			tagsJSON:     []byte(`[]`),
			expectedTags: []string{},
		},
		{
			name:         "nil tags field returns empty slice",
			tagsJSON:     nil,
			expectedTags: []string{},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ch := &channel.Channel{
				ID:            uuid.New(),
				WorkspaceID:   uuid.New(),
				Name:          "Test Channel",
				Visibility:    channel.ChannelVisibilityPrivate,
				Tags:          tc.tagsJSON,
				RetentionDays: 90,
				CreatedAt:     time.Now().UTC(),
				UpdatedAt:     time.Now().UTC(),
			}

			resp := toChannelResponse(ch)

			assert.Equal(t, tc.expectedTags, resp.Tags)
			assert.Equal(t, ch.ID.String(), resp.ID)
			assert.Equal(t, ch.WorkspaceID.String(), resp.WorkspaceID)
		})
	}
}

func TestToChannelResponse_DeviceID(t *testing.T) {
	devID := uuid.New()
	ch := &channel.Channel{
		ID:          uuid.New(),
		WorkspaceID: uuid.New(),
		DeviceID:    &devID,
		Name:        "Channel with Device",
		Visibility:  channel.ChannelVisibilityPublic,
		Tags:        []byte(`["tag1"]`),
		CreatedAt:   time.Now().UTC(),
		UpdatedAt:   time.Now().UTC(),
	}

	resp := toChannelResponse(ch)

	assert.NotNil(t, resp.DeviceID)
	assert.Equal(t, devID.String(), *resp.DeviceID)
	assert.Equal(t, []string{"tag1"}, resp.Tags)
}
