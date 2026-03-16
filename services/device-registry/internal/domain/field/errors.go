package field

import "errors"

var (
	ErrFieldNotFound    = errors.New("field not found")
	ErrInvalidName      = errors.New("name must not be empty")
	ErrInvalidPosition  = errors.New("position must be between 1 and 8")
	ErrInvalidFieldType = errors.New("invalid field type")
)
