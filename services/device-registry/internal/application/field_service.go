package application

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/greenlab/device-registry/internal/domain/field"
)

type FieldService struct {
	repo field.FieldRepository
}

func NewFieldService(repo field.FieldRepository) *FieldService {
	return &FieldService{repo: repo}
}

type CreateFieldInput struct {
	ChannelID   string
	Name        string
	Label       string
	Unit        string
	FieldType   string
	Position    int
	Description string
}

func (s *FieldService) CreateField(ctx context.Context, in CreateFieldInput) (*field.Field, error) {
	channelID, err := uuid.Parse(in.ChannelID)
	if err != nil {
		return nil, fmt.Errorf("CreateField.ParseChannelID: %w", err)
	}
	f, err := field.NewField(channelID, in.Name, in.Label, in.Unit, field.FieldType(in.FieldType), in.Position)
	if err != nil {
		return nil, fmt.Errorf("CreateField.NewField: %w", err)
	}
	f.Description = in.Description
	if err := s.repo.Create(ctx, f); err != nil {
		return nil, fmt.Errorf("CreateField.repo.Create: %w", err)
	}
	return f, nil
}

func (s *FieldService) GetField(ctx context.Context, id string) (*field.Field, error) {
	uid, err := uuid.Parse(id)
	if err != nil {
		return nil, fmt.Errorf("GetField.ParseID: %w", err)
	}
	return s.repo.GetByID(ctx, uid)
}

func (s *FieldService) ListFields(ctx context.Context, channelID string) ([]*field.Field, error) {
	cid, err := uuid.Parse(channelID)
	if err != nil {
		return nil, fmt.Errorf("ListFields.ParseChannelID: %w", err)
	}
	return s.repo.ListByChannel(ctx, cid)
}

type UpdateFieldInput struct {
	Name        string
	Label       string
	Unit        string
	Description string
}

func (s *FieldService) UpdateField(ctx context.Context, id string, in UpdateFieldInput) (*field.Field, error) {
	uid, err := uuid.Parse(id)
	if err != nil {
		return nil, fmt.Errorf("UpdateField.ParseID: %w", err)
	}
	f, err := s.repo.GetByID(ctx, uid)
	if err != nil {
		return nil, fmt.Errorf("UpdateField.GetByID: %w", err)
	}
	if in.Name != "" {
		if err := f.SetName(in.Name); err != nil {
			return nil, fmt.Errorf("UpdateField.SetName: %w", err)
		}
	}
	if in.Label != "" {
		f.SetLabel(in.Label)
	}
	if in.Unit != "" {
		f.SetUnit(in.Unit)
	}
	if in.Description != "" {
		f.SetDescription(in.Description)
	}
	if err := s.repo.Update(ctx, f); err != nil {
		return nil, fmt.Errorf("UpdateField.repo.Update: %w", err)
	}
	return f, nil
}

func (s *FieldService) DeleteField(ctx context.Context, id string) error {
	uid, err := uuid.Parse(id)
	if err != nil {
		return fmt.Errorf("DeleteField.ParseID: %w", err)
	}
	return s.repo.Delete(ctx, uid)
}
