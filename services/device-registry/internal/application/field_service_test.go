package application

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/greenlab/device-registry/internal/domain/field"
	mockfield "github.com/greenlab/device-registry/internal/mocks/field"
)

func newTestFieldService(t *testing.T) (*FieldService, *mockfield.MockFieldRepository) {
	t.Helper()
	repo := mockfield.NewMockFieldRepository(t)
	svc := NewFieldService(repo)
	return svc, repo
}

func TestCreateField(t *testing.T) {
	ctx := context.Background()

	t.Run("success", func(t *testing.T) {
		svc, repo := newTestFieldService(t)
		chID := uuid.New()
		repo.On("Create", ctx, mock.AnythingOfType("*field.Field")).Return(nil)

		f, err := svc.CreateField(ctx, CreateFieldInput{
			ChannelID: chID.String(),
			Name:      "temperature",
			Label:     "Temperature",
			Unit:      "°C",
			FieldType: "float",
			Position:  1,
		})
		require.NoError(t, err)
		assert.Equal(t, "temperature", f.Name)
		assert.Equal(t, field.FieldTypeFloat, f.FieldType)
		assert.Equal(t, 1, f.Position)
		repo.AssertExpectations(t)
	})

	t.Run("empty name returns domain error", func(t *testing.T) {
		svc, _ := newTestFieldService(t)
		f, err := svc.CreateField(ctx, CreateFieldInput{ChannelID: uuid.New().String(), Name: "", Position: 1})
		assert.ErrorIs(t, err, field.ErrInvalidName)
		assert.Nil(t, f)
	})

	t.Run("invalid position returns domain error", func(t *testing.T) {
		svc, _ := newTestFieldService(t)
		f, err := svc.CreateField(ctx, CreateFieldInput{ChannelID: uuid.New().String(), Name: "temp", Position: 0})
		assert.ErrorIs(t, err, field.ErrInvalidPosition)
		assert.Nil(t, f)
	})

	t.Run("invalid field type returns domain error", func(t *testing.T) {
		svc, _ := newTestFieldService(t)
		f, err := svc.CreateField(ctx, CreateFieldInput{ChannelID: uuid.New().String(), Name: "temp", FieldType: "bad", Position: 1})
		assert.ErrorIs(t, err, field.ErrInvalidFieldType)
		assert.Nil(t, f)
	})

	t.Run("empty field type defaults to float", func(t *testing.T) {
		svc, repo := newTestFieldService(t)
		repo.On("Create", ctx, mock.AnythingOfType("*field.Field")).Return(nil)

		f, err := svc.CreateField(ctx, CreateFieldInput{ChannelID: uuid.New().String(), Name: "temp", Position: 1})
		require.NoError(t, err)
		assert.Equal(t, field.FieldTypeFloat, f.FieldType)
		repo.AssertExpectations(t)
	})

	t.Run("invalid channel_id", func(t *testing.T) {
		svc, _ := newTestFieldService(t)
		f, err := svc.CreateField(ctx, CreateFieldInput{ChannelID: "not-uuid", Name: "temp", Position: 1})
		assert.Error(t, err)
		assert.Nil(t, f)
	})
}

func TestGetField(t *testing.T) {
	ctx := context.Background()

	t.Run("success", func(t *testing.T) {
		svc, repo := newTestFieldService(t)
		id := uuid.New()
		expected := &field.Field{ID: id, Name: "temp"}
		repo.On("GetByID", ctx, id).Return(expected, nil)

		f, err := svc.GetField(ctx, id.String())
		require.NoError(t, err)
		assert.Equal(t, expected, f)
		repo.AssertExpectations(t)
	})

	t.Run("not found", func(t *testing.T) {
		svc, repo := newTestFieldService(t)
		id := uuid.New()
		repo.On("GetByID", ctx, id).Return(nil, field.ErrFieldNotFound)

		f, err := svc.GetField(ctx, id.String())
		assert.ErrorIs(t, err, field.ErrFieldNotFound)
		assert.Nil(t, f)
		repo.AssertExpectations(t)
	})
}

func TestListFields(t *testing.T) {
	ctx := context.Background()

	t.Run("success", func(t *testing.T) {
		svc, repo := newTestFieldService(t)
		chID := uuid.New()
		expected := []*field.Field{{ID: uuid.New(), Name: "temp", Position: 1}}
		repo.On("ListByChannel", ctx, chID).Return(expected, nil)

		fields, err := svc.ListFields(ctx, chID.String())
		require.NoError(t, err)
		assert.Len(t, fields, 1)
		repo.AssertExpectations(t)
	})

	t.Run("invalid channel_id", func(t *testing.T) {
		svc, _ := newTestFieldService(t)
		fields, err := svc.ListFields(ctx, "bad-uuid")
		assert.Error(t, err)
		assert.Nil(t, fields)
	})
}

func TestUpdateField(t *testing.T) {
	ctx := context.Background()

	t.Run("success", func(t *testing.T) {
		svc, repo := newTestFieldService(t)
		id := uuid.New()
		existing := &field.Field{ID: id, Name: "old", Label: "Old Label"}
		repo.On("GetByID", ctx, id).Return(existing, nil)
		repo.On("Update", ctx, mock.AnythingOfType("*field.Field")).Return(nil)

		f, err := svc.UpdateField(ctx, id.String(), UpdateFieldInput{Name: "new", Label: "New Label"})
		require.NoError(t, err)
		assert.Equal(t, "new", f.Name)
		assert.Equal(t, "New Label", f.Label)
		repo.AssertExpectations(t)
	})

	t.Run("repo error on GetByID", func(t *testing.T) {
		svc, repo := newTestFieldService(t)
		id := uuid.New()
		repo.On("GetByID", ctx, id).Return(nil, errors.New("db error"))

		f, err := svc.UpdateField(ctx, id.String(), UpdateFieldInput{Name: "new"})
		assert.Error(t, err)
		assert.Nil(t, f)
		repo.AssertExpectations(t)
	})
}

func TestDeleteField(t *testing.T) {
	ctx := context.Background()

	t.Run("success", func(t *testing.T) {
		svc, repo := newTestFieldService(t)
		id := uuid.New()
		repo.On("Delete", ctx, id).Return(nil)

		err := svc.DeleteField(ctx, id.String())
		require.NoError(t, err)
		repo.AssertExpectations(t)
	})

	t.Run("invalid id", func(t *testing.T) {
		svc, _ := newTestFieldService(t)
		err := svc.DeleteField(ctx, "not-uuid")
		assert.Error(t, err)
	})
}
