package audit

import "errors"

var (
	ErrEventNotFound  = errors.New("audit event not found")
	ErrInvalidEventID = errors.New("invalid event ID")
)
