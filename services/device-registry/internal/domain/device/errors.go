package device

import "errors"

var (
	ErrDeviceNotFound      = errors.New("device not found")
	ErrDeviceAlreadyDeleted = errors.New("device already deleted")
	ErrInvalidName         = errors.New("name must not be empty")
	ErrInvalidStatus       = errors.New("invalid device status")
	ErrCacheMiss           = errors.New("cache miss")
)
