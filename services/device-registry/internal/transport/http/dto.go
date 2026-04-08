package http

import (
	"encoding/json"
	"time"
)

// Provision DTOs
type ProvisionDeviceRequest struct {
	WorkspaceID string `json:"workspace_id" validate:"required"`
	Name        string `json:"name"         validate:"required"`
	Description string `json:"description"`
}

type ProvisionChannelRequest struct {
	Name        string `json:"name"       validate:"required"`
	Description string `json:"description"`
	Visibility  string `json:"visibility"`
}

type ProvisionFieldRequest struct {
	Name      string `json:"name"       validate:"required"`
	Label     string `json:"label"`
	Unit      string `json:"unit"`
	FieldType string `json:"field_type"`
	Position  int    `json:"position"   validate:"required"`
}

type ProvisionRequest struct {
	Device  ProvisionDeviceRequest  `json:"device"   validate:"required"`
	Channel ProvisionChannelRequest `json:"channel"  validate:"required"`
	Fields  []ProvisionFieldRequest `json:"fields"`
}

type ProvisionResponse struct {
	Device  *DeviceResponse   `json:"device"`
	Channel *ChannelResponse  `json:"channel"`
	Fields  []*FieldResponse  `json:"fields"`
}

// Device DTOs
type CreateDeviceRequest struct {
	WorkspaceID       string   `json:"workspace_id"       validate:"required"`
	Name              string   `json:"name"               validate:"required"`
	Description       string   `json:"description"`
	Lat               *float64 `json:"lat"               validate:"omitempty,min=-90,max=90"`
	Lng               *float64 `json:"lng"               validate:"omitempty,min=-180,max=180"`
	LocationAddress   string   `json:"location_address"`
	ChannelName       string   `json:"channel_name"`
	ChannelVisibility string   `json:"channel_visibility" validate:"omitempty,oneof=public private"`
}

type UpdateDeviceRequest struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Status      string `json:"status"`
}

type DeviceResponse struct {
	ID              string          `json:"id"`
	WorkspaceID     string          `json:"workspace_id"`
	Name            string          `json:"name"`
	Description     string          `json:"description"`
	APIKey          string          `json:"api_key,omitempty"`
	Status          string          `json:"status"`
	Metadata        json.RawMessage `json:"metadata,omitempty" swaggertype:"object"`
	Lat             *float64        `json:"lat,omitempty"`
	Lng             *float64        `json:"lng,omitempty"`
	LocationAddress string          `json:"location_address,omitempty"`
	LastSeenAt      *time.Time      `json:"last_seen_at"`
	CreatedAt       time.Time       `json:"created_at"`
	UpdatedAt       time.Time       `json:"updated_at"`
}

// Channel DTOs
type CreateChannelRequest struct {
	WorkspaceID   string  `json:"workspace_id"   validate:"required"`
	DeviceID      *string `json:"device_id"`
	Name          string  `json:"name"           validate:"required"`
	Description   string  `json:"description"`
	Visibility    string  `json:"visibility"`
	RetentionDays int     `json:"retention_days"`
}

type UpdateChannelRequest struct {
	Name          string `json:"name"`
	Description   string `json:"description"`
	Visibility    string `json:"visibility"`
	RetentionDays int    `json:"retention_days"`
}

type ChannelResponse struct {
	ID            string    `json:"id"`
	ShortID       int       `json:"short_id"`
	WorkspaceID   string    `json:"workspace_id"`
	DeviceID      *string   `json:"device_id,omitempty"`
	Name          string    `json:"name"`
	Description   string    `json:"description"`
	Visibility    string    `json:"visibility"`
	RetentionDays int       `json:"retention_days"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

// Field DTOs
type CreateFieldRequest struct {
	ChannelID   string `json:"channel_id"  validate:"required"`
	Name        string `json:"name"        validate:"required"`
	Label       string `json:"label"`
	Unit        string `json:"unit"`
	FieldType   string `json:"field_type"`
	Position    *int   `json:"position"    validate:"required"`
	Description string `json:"description"`
}

type UpdateFieldRequest struct {
	Name        string `json:"name"`
	Label       string `json:"label"`
	Unit        string `json:"unit"`
	Description string `json:"description"`
}

type FieldResponse struct {
	ID          string    `json:"id"`
	ChannelID   string    `json:"channel_id"`
	Name        string    `json:"name"`
	Label       string    `json:"label"`
	Unit        string    `json:"unit"`
	FieldType   string    `json:"field_type"`
	Position    int       `json:"position"`
	Description string    `json:"description"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}
