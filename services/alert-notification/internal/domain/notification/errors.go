package notification

import "errors"

var (
	ErrNotificationNotFound  = errors.New("notification not found")
	ErrInvalidWorkspace      = errors.New("invalid workspace_id")
	ErrInvalidNotificationID = errors.New("invalid notification ID")
)
