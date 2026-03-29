package middleware

import (
	"context"
	"errors"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/greenlab/shared/pkg/apierr"
	"github.com/greenlab/shared/pkg/response"
)

const (
	ContextKeyUserID   = "user_id"
	ContextKeyTenantID = "tenant_id"
	ContextKeyEmail    = "email"
	ContextKeyRoles    = "roles"
	ContextKeyAPIKey   = "api_key"
)

// Claims represents JWT claims for a user.
type Claims struct {
	UserID   string   `json:"uid"`
	TenantID string   `json:"tid"`
	Email    string   `json:"email"`
	Roles    []string `json:"roles"`
	jwt.RegisteredClaims
}

// JWTAuth returns a Gin middleware that validates RS256 JWT tokens.
func JWTAuth(publicKey interface{}) gin.HandlerFunc {
	return func(c *gin.Context) {
		raw, err := extractBearerToken(c)
		if err != nil {
			response.Abort(c, apierr.Unauthorized(err.Error()))
			return
		}

		claims := &Claims{}
		token, err := jwt.ParseWithClaims(raw, claims, func(t *jwt.Token) (interface{}, error) {
			if _, ok := t.Method.(*jwt.SigningMethodRSA); !ok {
				return nil, errors.New("unexpected signing method")
			}
			return publicKey, nil
		})
		if err != nil || !token.Valid {
			response.Abort(c, apierr.Unauthorized("invalid or expired token"))
			return
		}

		c.Set(ContextKeyUserID, claims.UserID)
		c.Set(ContextKeyTenantID, claims.TenantID)
		c.Set(ContextKeyEmail, claims.Email)
		c.Set(ContextKeyRoles, claims.Roles)
		c.Next()
	}
}

// APIKeyAuth returns a Gin middleware that validates API keys via a context-aware lookup function.
// The lookup receives the request context so cancellations and deadlines propagate to
// downstream Redis and Postgres calls.
// The lookup function receives the key and should return (deviceID, channelID, err).
func APIKeyAuth(lookup func(ctx context.Context, key string) (deviceID, channelID string, err error)) gin.HandlerFunc {
	return func(c *gin.Context) {
		key := c.GetHeader("X-API-Key")
		if key == "" {
			key = c.Query("api_key")
		}
		if key == "" {
			response.Abort(c, apierr.Unauthorized("missing API key"))
			return
		}

		deviceID, channelID, err := lookup(c.Request.Context(), key)
		if err != nil {
			response.Abort(c, apierr.Unauthorized("invalid API key"))
			return
		}

		c.Set(ContextKeyAPIKey, key)
		c.Set("device_id", deviceID)
		c.Set("channel_id", channelID)
		c.Next()
	}
}

// RequireRoles returns middleware that enforces at least one of the given roles.
func RequireRoles(roles ...string) gin.HandlerFunc {
	return func(c *gin.Context) {
		userRoles, exists := c.Get(ContextKeyRoles)
		if !exists {
			response.Abort(c, apierr.Forbidden("no roles found"))
			return
		}

		roleSlice, ok := userRoles.([]string)
		if !ok {
			response.Abort(c, apierr.Forbidden("invalid role format"))
			return
		}

		roleSet := make(map[string]struct{}, len(roleSlice))
		for _, r := range roleSlice {
			roleSet[r] = struct{}{}
		}

		for _, required := range roles {
			if _, ok := roleSet[required]; ok {
				c.Next()
				return
			}
		}

		response.Abort(c, apierr.Forbidden("insufficient permissions"))
	}
}

func extractBearerToken(c *gin.Context) (string, error) {
	header := c.GetHeader("Authorization")
	if header != "" {
		parts := strings.SplitN(header, " ", 2)
		if len(parts) != 2 || !strings.EqualFold(parts[0], "bearer") {
			return "", errors.New("invalid Authorization header format")
		}
		if parts[1] == "" {
			return "", errors.New("empty token")
		}
		return parts[1], nil
	}
	// Fall back to ?token= query param for WebSocket connections
	// (browsers cannot set custom headers for WebSocket upgrades)
	if token := c.Query("token"); token != "" {
		return token, nil
	}
	return "", errors.New("missing Authorization header")
}

// GetUserID is a helper that extracts the user ID from context.
func GetUserID(c *gin.Context) (string, bool) {
	v, exists := c.Get(ContextKeyUserID)
	if !exists {
		return "", false
	}
	s, ok := v.(string)
	return s, ok
}

// GetTenantID is a helper that extracts the tenant ID from context.
func GetTenantID(c *gin.Context) (string, bool) {
	v, exists := c.Get(ContextKeyTenantID)
	if !exists {
		return "", false
	}
	s, ok := v.(string)
	return s, ok
}

// OptionalJWTAuth tries to authenticate but does not abort if no token is present.
func OptionalJWTAuth(publicKey interface{}) gin.HandlerFunc {
	return func(c *gin.Context) {
		raw, err := extractBearerToken(c)
		if err != nil {
			c.Next()
			return
		}

		claims := &Claims{}
		token, err := jwt.ParseWithClaims(raw, claims, func(t *jwt.Token) (interface{}, error) {
			if _, ok := t.Method.(*jwt.SigningMethodRSA); !ok {
				return nil, errors.New("unexpected signing method")
			}
			return publicKey, nil
		})
		if err != nil || !token.Valid {
			c.Next()
			return
		}

		c.Set(ContextKeyUserID, claims.UserID)
		c.Set(ContextKeyTenantID, claims.TenantID)
		c.Set(ContextKeyEmail, claims.Email)
		c.Set(ContextKeyRoles, claims.Roles)
		c.Next()
	}
}
