package http

import (
	"crypto/rsa"

	"github.com/gin-gonic/gin"
	ginSwagger "github.com/swaggo/gin-swagger"
	swaggerFiles "github.com/swaggo/files"
	sharedMiddleware "github.com/greenlab/shared/pkg/middleware"
)

// NewRouter wires handlers and attaches middleware.
func NewRouter(videoH *VideoHandler, auditH *AuditHandler, publicKey *rsa.PublicKey) *gin.Engine {
	r := gin.New()
	r.Use(gin.Logger())
	r.Use(gin.Recovery())
	r.Use(securityHeaders())
	r.Use(sharedMiddleware.RequestID())

	r.GET("/health", videoH.Health)
	r.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

	v1 := r.Group("/api/v1")
	v1.Use(sharedMiddleware.JWTAuth(publicKey))
	{
		streams := v1.Group("/streams")
		{
			streams.POST("", videoH.CreateStream)
			streams.GET("", videoH.ListStreams)
			streams.GET("/:id", videoH.GetStream)
			streams.PATCH("/:id/status", videoH.UpdateStreamStatus)
			streams.GET("/:id/upload-url", videoH.GetUploadURL)
			streams.GET("/:id/download-url", videoH.GetDownloadURL)
			streams.DELETE("/:id", videoH.DeleteStream)
		}

		auditGroup := v1.Group("/audit")
		{
			auditGroup.GET("/events", auditH.ListByTenant)
			auditGroup.GET("/events/resource", auditH.ListByResource)
			auditGroup.GET("/events/:id", auditH.GetEvent)
		}
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
