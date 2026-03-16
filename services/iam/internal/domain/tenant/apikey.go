package tenant

import "time"

// APIKey represents an org-level API key for programmatic access.
type APIKey struct {
	ID        string     `db:"id"         json:"id"`
	TenantID  string     `db:"tenant_id"  json:"tenant_id"`
	UserID    string     `db:"user_id"    json:"user_id"`
	Name      string     `db:"name"       json:"name"`
	KeyPrefix string     `db:"key_prefix" json:"key_prefix"`
	KeyHash   string     `db:"key_hash"   json:"-"`
	Scopes    []string   `db:"scopes"     json:"scopes"`
	CreatedAt time.Time  `db:"created_at" json:"created_at"`
	LastUsed  *time.Time `db:"last_used"  json:"last_used,omitempty"`
}
