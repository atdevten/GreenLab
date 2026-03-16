package alert

import "errors"

var (
	ErrRuleNotFound      = errors.New("rule not found")
	ErrInvalidRuleID     = errors.New("invalid rule ID")
	ErrInvalidChannelID  = errors.New("invalid channel_id")
	ErrInvalidWorkspace  = errors.New("invalid workspace_id")
)
