package field

import (
	"strings"
	"time"

	"github.com/google/uuid"
)

// FieldType defines the data type of a field value.
type FieldType string

const (
	FieldTypeFloat   FieldType = "float"
	FieldTypeInteger FieldType = "integer"
	FieldTypeString  FieldType = "string"
	FieldTypeBoolean FieldType = "boolean"
)

// Field defines a named measurement within a Channel.
type Field struct {
	ID          uuid.UUID
	ChannelID   uuid.UUID
	Name        string
	Label       string
	Unit        string
	FieldType   FieldType
	Position    int // ordering within channel (1-8)
	Description string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// SetName validates and sets the field name.
func (f *Field) SetName(name string) error {
	if strings.TrimSpace(name) == "" {
		return ErrInvalidName
	}
	f.Name = strings.TrimSpace(name)
	f.UpdatedAt = time.Now().UTC()
	return nil
}

// SetLabel sets the field label and stamps UpdatedAt.
func (f *Field) SetLabel(label string) {
	f.Label = label
	f.UpdatedAt = time.Now().UTC()
}

// SetUnit sets the field unit and stamps UpdatedAt.
func (f *Field) SetUnit(unit string) {
	f.Unit = unit
	f.UpdatedAt = time.Now().UTC()
}

// SetDescription sets the field description and stamps UpdatedAt.
func (f *Field) SetDescription(description string) {
	f.Description = description
	f.UpdatedAt = time.Now().UTC()
}

// NewField creates a new Field with validation.
func NewField(channelID uuid.UUID, name, label, unit string, fieldType FieldType, position int) (*Field, error) {
	if strings.TrimSpace(name) == "" {
		return nil, ErrInvalidName
	}
	if position < 1 || position > 8 {
		return nil, ErrInvalidPosition
	}
	if fieldType == "" {
		fieldType = FieldTypeFloat
	}
	switch fieldType {
	case FieldTypeFloat, FieldTypeInteger, FieldTypeString, FieldTypeBoolean:
		// valid
	default:
		return nil, ErrInvalidFieldType
	}
	now := time.Now().UTC()
	return &Field{
		ID:        uuid.New(),
		ChannelID: channelID,
		Name:      name,
		Label:     label,
		Unit:      unit,
		FieldType: fieldType,
		Position:  position,
		CreatedAt: now,
		UpdatedAt: now,
	}, nil
}
