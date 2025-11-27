package routes

import (
	"github.com/gin-gonic/gin"
	"github.com/mot0x0/gopi/internal/delivery/http/handlers"
)

func RegisterAuthRoutes(router *gin.RouterGroup, authHandler *handlers.AuthHandler) {
	public := router.Group("/auth")
	{
		public.POST("/login", authHandler.Login)
		public.POST("/refresh", authHandler.Refresh)
		public.POST("/logout", authHandler.Logout)
	}
}
