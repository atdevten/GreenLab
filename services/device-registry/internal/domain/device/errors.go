package device

import "errors"

var (
	ErrDeviceNotFound = errors.New("device not found")
	ErrInvalidName    = errors.New("name must not be empty")
	ErrInvalidStatus  = errors.New("invalid device status")
	ErrCacheMiss      = errors.New("cache miss")
)
