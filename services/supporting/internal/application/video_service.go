package application

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/greenlab/supporting/internal/domain/video"
)

// Storage defines object storage operations.
type Storage interface {
	GenerateUploadURL(ctx context.Context, key string, contentType string) (string, error)
	GenerateDownloadURL(ctx context.Context, key string) (string, error)
	Delete(ctx context.Context, key string) error
}

// VideoService implements video stream management.
type VideoService struct {
	repo    video.StreamRepository
	storage Storage
	logger  *slog.Logger
}

func NewVideoService(repo video.StreamRepository, storage Storage, logger *slog.Logger) *VideoService {
	return &VideoService{repo: repo, storage: storage, logger: logger}
}

type CreateStreamInput struct {
	DeviceID    string
	WorkspaceID string
	Name        string
	Description string
	Protocol    string
	SourceURL   string
}

func (s *VideoService) CreateStream(ctx context.Context, in CreateStreamInput) (*video.Stream, error) {
	deviceID, err := uuid.Parse(in.DeviceID)
	if err != nil {
		return nil, video.ErrInvalidDeviceID
	}
	wsID, err := uuid.Parse(in.WorkspaceID)
	if err != nil {
		return nil, video.ErrInvalidWorkspaceID
	}
	protocol := video.StreamProtocol(in.Protocol)
	if !protocol.IsValid() {
		return nil, video.ErrInvalidProtocol
	}
	stream := video.NewStream(deviceID, wsID, in.Name, in.Description, protocol, in.SourceURL)

	if err := s.repo.Create(ctx, stream); err != nil {
		return nil, err
	}
	return stream, nil
}

func (s *VideoService) GetStream(ctx context.Context, id string) (*video.Stream, error) {
	uid, err := uuid.Parse(id)
	if err != nil {
		return nil, video.ErrInvalidStreamID
	}
	return s.repo.GetByID(ctx, uid)
}

func (s *VideoService) ListStreams(ctx context.Context, deviceID string, limit, offset int) ([]*video.Stream, int64, error) {
	uid, err := uuid.Parse(deviceID)
	if err != nil {
		return nil, 0, video.ErrInvalidDeviceID
	}
	return s.repo.ListByDevice(ctx, uid, limit, offset)
}

func (s *VideoService) UpdateStreamStatus(ctx context.Context, id string, status video.StreamStatus) (*video.Stream, error) {
	if !status.IsValid() {
		return nil, video.ErrInvalidStatus
	}
	uid, err := uuid.Parse(id)
	if err != nil {
		return nil, video.ErrInvalidStreamID
	}
	stream, err := s.repo.GetByID(ctx, uid)
	if err != nil {
		return nil, err
	}
	stream.Status = status
	stream.UpdatedAt = time.Now().UTC()
	if err := s.repo.Update(ctx, stream); err != nil {
		return nil, err
	}
	return stream, nil
}

func (s *VideoService) GetUploadURL(ctx context.Context, streamID, contentType string) (string, error) {
	uid, err := uuid.Parse(streamID)
	if err != nil {
		return "", video.ErrInvalidStreamID
	}
	stream, err := s.repo.GetByID(ctx, uid)
	if err != nil {
		return "", err
	}
	key := fmt.Sprintf("videos/%s/%s", stream.ID.String(), uuid.New().String())
	stream.StorageKey = key
	if err := s.repo.Update(ctx, stream); err != nil {
		s.logger.Error("update stream storage key", "stream_id", streamID, "error", err)
	}
	return s.storage.GenerateUploadURL(ctx, key, contentType)
}

func (s *VideoService) GetDownloadURL(ctx context.Context, streamID string) (string, error) {
	uid, err := uuid.Parse(streamID)
	if err != nil {
		return "", video.ErrInvalidStreamID
	}
	stream, err := s.repo.GetByID(ctx, uid)
	if err != nil {
		return "", err
	}
	if stream.StorageKey == "" {
		return "", video.ErrNoRecording
	}
	return s.storage.GenerateDownloadURL(ctx, stream.StorageKey)
}

func (s *VideoService) DeleteStream(ctx context.Context, id string) error {
	uid, err := uuid.Parse(id)
	if err != nil {
		return video.ErrInvalidStreamID
	}
	stream, err := s.repo.GetByID(ctx, uid)
	if err != nil {
		return err
	}
	if stream.StorageKey != "" {
		if err := s.storage.Delete(ctx, stream.StorageKey); err != nil {
			// Best-effort S3 cleanup — log but continue with DB deletion.
			s.logger.Error("delete stream s3 object", "stream_id", id, "key", stream.StorageKey, "error", err)
		}
	}
	return s.repo.Delete(ctx, uid)
}
