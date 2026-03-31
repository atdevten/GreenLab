package http

import (
	"crypto/rsa"

	"github.com/gin-gonic/gin"
	ginSwagger "github.com/swaggo/gin-swagger"
	swaggerFiles "github.com/swaggo/files"
	sharedMiddleware "github.com/greenlab/shared/pkg/middleware"
)

func securityHeaders() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("X-Content-Type-Options", "nosniff")
		c.Header("X-Frame-Options", "DENY")
		c.Header("X-XSS-Protection", "1; mode=block")
		c.Header("Referrer-Policy", "strict-origin-when-cross-origin")
		c.Next()
	}
}

func NewRouter(deviceH *DeviceHandler, channelH *ChannelHandler, fieldH *FieldHandler, internalH *InternalHandler, provisionH *ProvisionHandler, publicKey *rsa.PublicKey) *gin.Engine {
	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(sharedMiddleware.RequestID())
	r.Use(securityHeaders())
	r.GET("/health", deviceH.Health)
	r.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

	v1 := r.Group("/api/v1")
	v1.Use(sharedMiddleware.JWTAuth(publicKey))
	{
		devices := v1.Group("/devices")
		{
			devices.POST("/provision", provisionH.Provision)
			devices.POST("", deviceH.CreateDevice)
			devices.GET("", deviceH.ListDevices)
			devices.GET("/:id", deviceH.GetDevice)
			devices.PUT("/:id", deviceH.UpdateDevice)
			devices.DELETE("/:id", deviceH.DeleteDevice)
			devices.POST("/:id/rotate-key", deviceH.RotateAPIKey)
		}

		workspaces := v1.Group("/workspaces")
		{
			workspaces.GET("/:id/devices", deviceH.ListByWorkspace)
		}

		channels := v1.Group("/channels")
		{
			channels.POST("", channelH.CreateChannel)
			channels.GET("", channelH.ListChannels)
			channels.GET("/:id", channelH.GetChannel)
			channels.PUT("/:id", channelH.UpdateChannel)
			channels.DELETE("/:id", channelH.DeleteChannel)
		}

		fields := v1.Group("/fields")
		{
			fields.POST("", fieldH.CreateField)
			fields.GET("", fieldH.ListFields)
			fields.GET("/:id", fieldH.GetField)
			fields.PUT("/:id", fieldH.UpdateField)
			fields.DELETE("/:id", fieldH.DeleteField)
		}
	}
	internal := r.Group("/internal")
	{
		internal.POST("/validate-api-key", internalH.ValidateAPIKey)
	}

	// Device-facing schema endpoint — authenticated via X-API-Key header.
	deviceV1 := r.Group("/v1")
	{
		deviceV1.GET("/channels/:id/schema", internalH.GetChannelSchema)
	}

	return r
}
