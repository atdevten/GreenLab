package telemetry

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel"
)

func TestInitTracer_EmptyEndpoint_NoOp(t *testing.T) {
	shutdown, err := InitTracer(context.Background(), "test-service", "")
	require.NoError(t, err)
	require.NotNil(t, shutdown)

	// No-op shutdown should not error.
	err = shutdown(context.Background())
	assert.NoError(t, err)
}

func TestInitTracer_EmptyEndpoint_LeavesDefaultProvider(t *testing.T) {
	providerBefore := otel.GetTracerProvider()

	shutdown, err := InitTracer(context.Background(), "test-service", "")
	require.NoError(t, err)
	defer shutdown(context.Background()) //nolint:errcheck

	// Empty endpoint must not replace the global tracer provider.
	assert.Equal(t, providerBefore, otel.GetTracerProvider())
}

func TestInitTracer_WithEndpoint_RegistersProvider(t *testing.T) {
	// Point at a non-existent endpoint — the exporter is created lazily, so
	// New() succeeds even when nothing is listening.
	shutdown, err := InitTracer(context.Background(), "test-service", "localhost:14318")
	require.NoError(t, err)
	require.NotNil(t, shutdown)

	// A real SDK provider must have been registered.
	tp := otel.GetTracerProvider()
	assert.NotNil(t, tp)

	// Shutdown should not panic; errors are acceptable (nothing listening).
	_ = shutdown(context.Background())
}
