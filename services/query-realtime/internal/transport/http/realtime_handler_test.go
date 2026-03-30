package http

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAllowedOrigins_CommaList(t *testing.T) {
	t.Setenv("ALLOWED_ORIGINS", "https://app.example.com, https://dash.example.com")
	t.Setenv("FRONTEND_URL", "")

	origins := allowedOrigins()

	assert.Contains(t, origins, "https://app.example.com")
	assert.Contains(t, origins, "https://dash.example.com")
	assert.Len(t, origins, 2)
}

func TestAllowedOrigins_Wildcard(t *testing.T) {
	t.Setenv("ALLOWED_ORIGINS", "*")
	t.Setenv("FRONTEND_URL", "")

	origins := allowedOrigins()

	_, ok := origins["*"]
	assert.True(t, ok)
}

func TestAllowedOrigins_FallbackToFrontendURL(t *testing.T) {
	t.Setenv("ALLOWED_ORIGINS", "")
	t.Setenv("FRONTEND_URL", "https://frontend.example.com")

	origins := allowedOrigins()

	assert.Contains(t, origins, "https://frontend.example.com")
	assert.Len(t, origins, 1)
}

func TestAllowedOrigins_EmptyRejectsAll(t *testing.T) {
	t.Setenv("ALLOWED_ORIGINS", "")
	t.Setenv("FRONTEND_URL", "")

	origins := allowedOrigins()

	assert.Empty(t, origins)
}

func TestAllowedOrigins_TrimsWhitespace(t *testing.T) {
	t.Setenv("ALLOWED_ORIGINS", "  https://a.example.com  ,  https://b.example.com  ")
	t.Setenv("FRONTEND_URL", "")

	origins := allowedOrigins()

	assert.Contains(t, origins, "https://a.example.com")
	assert.Contains(t, origins, "https://b.example.com")
	assert.Len(t, origins, 2)
}
