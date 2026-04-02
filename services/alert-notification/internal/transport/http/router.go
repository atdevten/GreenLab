package http

import (
	"github.com/gin-gonic/gin"
	ginSwagger "github.com/swaggo/gin-swagger"
	swaggerFiles "github.com/swaggo/files"
	sharedMiddleware "github.com/greenlab/shared/pkg/middleware"
)

// NewRouter wires handlers and attaches middleware.
// publicKey is an *rsa.PublicKey passed as interface{} to avoid importing
// crypto/rsa at the transport layer; the jwt library handles the type
// assertion internally.
func NewRouter(alertH *AlertHandler, notifH *NotificationHandler, publicKey interface{}) *gin.Engine {
	r := gin.New()
	r.Use(gin.Logger())
	r.Use(gin.Recovery())
	r.Use(securityHeaders())
	r.Use(sharedMiddleware.RequestID())
	r.GET("/health", alertH.Health)
	r.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

	v1 := r.Group("/api/v1")
	v1.Use(sharedMiddleware.JWTAuth(publicKey))
	{
		alertRules := v1.Group("/alert-rules")
		{
			alertRules.POST("", alertH.CreateRule)
			alertRules.GET("", alertH.ListRules)
			alertRules.GET("/:id", alertH.GetRule)
			alertRules.PUT("/:id", alertH.UpdateRule)
			alertRules.DELETE("/:id", alertH.DeleteRule)
			alertRules.GET("/:id/deliveries", alertH.ListDeliveries)
			alertRules.POST("/:id/verify-signature", alertH.VerifySignature)
		}

		notifications := v1.Group("/notifications")
		{
			notifications.POST("", notifH.SendNotification)
			notifications.GET("", notifH.ListNotifications)
			// read-all must be registered before /:id to avoid routing conflict
			notifications.POST("/read-all", notifH.MarkAllRead)
			notifications.GET("/:id", notifH.GetNotification)
			notifications.PATCH("/:id/read", notifH.MarkRead)
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
