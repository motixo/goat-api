package routes

import (
	"github.com/gin-gonic/gin"
	"github.com/mot0x0/gopi/internal/delivery/http/handlers"
	"github.com/mot0x0/gopi/internal/delivery/http/middleware"
)

func RegisterAuthRoutes(router *gin.RouterGroup, authHandler *handlers.AuthHandler, authMiddleware *middleware.AuthMiddleware) {
	public := router.Group("/auth")
	{
		public.POST("/login", authHandler.Login)
		public.POST("/signup", authHandler.Register)
		public.POST("/refresh", authHandler.Refresh)
	}

	private := router.Group("/auth")
	private.Use(authMiddleware.Required())
	{
		private.POST("/logout", authHandler.Logout)
	}

}
