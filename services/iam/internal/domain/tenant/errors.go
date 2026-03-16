package tenant

import "errors"

var (
	ErrOrgNotFound        = errors.New("org not found")
	ErrWorkspaceNotFound  = errors.New("workspace not found")
	ErrSlugAlreadyTaken   = errors.New("slug already taken")
	ErrInvalidName        = errors.New("name must not be empty")
	ErrInvalidSlug        = errors.New("slug must be lowercase alphanumeric and hyphens only, no leading/trailing hyphens")
	ErrMemberNotFound     = errors.New("workspace member not found")
	ErrMemberAlreadyExists = errors.New("user is already a member of this workspace")
	ErrInvalidRole        = errors.New("role must be one of: owner, admin, member, viewer")
	ErrAPIKeyNotFound     = errors.New("api key not found")
)
