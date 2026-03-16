package http

import (
	"github.com/gin-gonic/gin"
	ginSwagger "github.com/swaggo/gin-swagger"
	swaggerFiles "github.com/swaggo/files"
	sharedMiddleware "github.com/greenlab/shared/pkg/middleware"
)

// NewRouter wires handlers to routes and attaches middleware.
// publicKey is an *rsa.PublicKey passed as interface{} to avoid importing
// crypto/rsa at the transport layer; the jwt library performs the type
// assertion internally via the keyfunc.
func NewRouter(queryH *QueryHandler, realtimeH *RealtimeHandler, publicKey interface{}) *gin.Engine {
	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(securityHeaders())
	r.Use(sharedMiddleware.RequestID())
	r.GET("/health", queryH.Health)
	r.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

	v1 := r.Group("/api/v1")
	{
		// All query endpoints require a valid JWT.
		queryGroup := v1.Group("")
		queryGroup.Use(sharedMiddleware.JWTAuth(publicKey))
		{
			queryGroup.GET("/query", queryH.Query)
			queryGroup.GET("/query/latest", queryH.QueryLatest)
		}

		// WebSocket and SSE require a valid JWT.
		// OptionalJWTAuth was removed because it allowed unauthenticated clients
		// to subscribe to any channel stream. Per-channel authorization (verifying
		// the caller owns the channel) is a TODO inside each handler.
		v1.GET("/ws", sharedMiddleware.JWTAuth(publicKey), realtimeH.WebSocket)
		v1.GET("/sse", sharedMiddleware.JWTAuth(publicKey), realtimeH.SSE)

		// Stats requires auth.
		v1.GET("/stats", sharedMiddleware.JWTAuth(publicKey), realtimeH.Stats)
	}
	return r
}

// securityHeaders adds baseline security response headers to every response.
func securityHeaders() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("X-Content-Type-Options", "nosniff")
		c.Header("X-Frame-Options", "DENY")
		c.Header("X-XSS-Protection", "1; mode=block")
		c.Next()
	}
}
