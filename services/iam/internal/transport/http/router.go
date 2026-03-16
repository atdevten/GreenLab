package http

import (
	"crypto/rsa"

	"github.com/gin-gonic/gin"
	ginSwagger "github.com/swaggo/gin-swagger"
	swaggerFiles "github.com/swaggo/files"
	sharedMiddleware "github.com/greenlab/shared/pkg/middleware"
	"go.uber.org/zap"
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

func NewRouter(authH *AuthHandler, tenantH *TenantHandler, publicKey *rsa.PublicKey, log *zap.Logger) *gin.Engine {
	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(sharedMiddleware.RequestID())
	r.Use(sharedMiddleware.Logger(log))
	r.Use(securityHeaders())

	r.GET("/health", authH.Health)
	r.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

	v1 := r.Group("/api/v1")
	{
		// Auth routes
		authGroup := v1.Group("/auth")
		{
			// Public endpoints
			authGroup.POST("/signup", authH.Signup)
			authGroup.POST("/register", authH.Register)
			authGroup.POST("/login", authH.Login)
			authGroup.POST("/refresh", authH.Refresh)
			authGroup.POST("/forgot-password", authH.ForgotPassword)
			authGroup.POST("/reset-password", authH.ResetPassword)
			authGroup.POST("/verify-email", authH.VerifyEmail)

			// Protected endpoints
			protected := authGroup.Group("")
			protected.Use(sharedMiddleware.JWTAuth(publicKey))
			{
				protected.POST("/logout", authH.Logout)
				protected.GET("/me", authH.GetMe)
				protected.PUT("/me", authH.UpdateMe)
				protected.PUT("/me/password", authH.ChangePassword)
			}
		}

		// Tenant routes (all protected)
		tenantGroup := v1.Group("")
		tenantGroup.Use(sharedMiddleware.JWTAuth(publicKey))
		{
			orgs := tenantGroup.Group("/orgs")
			{
				orgs.POST("", tenantH.CreateOrg)
				orgs.GET("", tenantH.ListOrgs)
				orgs.GET("/:id", tenantH.GetOrg)
				orgs.PUT("/:id", tenantH.UpdateOrg)
				orgs.DELETE("/:id", tenantH.DeleteOrg)
				orgs.GET("/:id/workspaces", tenantH.ListWorkspaces)
			}
			ws := tenantGroup.Group("/workspaces")
			{
				ws.POST("", tenantH.CreateWorkspace)
				ws.PUT("/:id", tenantH.UpdateWorkspace)
				ws.DELETE("/:id", tenantH.DeleteWorkspace)
				ws.GET("/:id/members", tenantH.ListMembers)
				ws.POST("/:id/members", tenantH.AddMember)
				ws.PUT("/:id/members/:userId", tenantH.UpdateMember)
				ws.DELETE("/:id/members/:userId", tenantH.RemoveMember)
			}
			apiKeys := tenantGroup.Group("/api-keys")
			{
				apiKeys.GET("", tenantH.ListAPIKeys)
				apiKeys.POST("", tenantH.CreateAPIKey)
				apiKeys.DELETE("/:id", tenantH.DeleteAPIKey)
			}
		}
	}

	return r
}
