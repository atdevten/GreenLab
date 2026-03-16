package tenant

import "time"

// ValidRoles is the allowed set of workspace member roles.
var ValidRoles = map[string]bool{
	"owner":  true,
	"admin":  true,
	"member": true,
	"viewer": true,
}

// WorkspaceMember represents a user's membership in a workspace.
type WorkspaceMember struct {
	ID          string    `db:"id"           json:"id"`
	WorkspaceID string    `db:"workspace_id" json:"workspace_id"`
	UserID      string    `db:"user_id"      json:"user_id"`
	Name        string    `db:"name"         json:"name"`
	Email       string    `db:"email"        json:"email"`
	Role        string    `db:"role"         json:"role"`
	JoinedAt    time.Time `db:"joined_at"    json:"joined_at"`
}
