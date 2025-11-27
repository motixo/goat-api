package routes

import (
	"github.com/gin-gonic/gin"
	"github.com/mot0x0/gopi/internal/delivery/http/handlers"
	"github.com/mot0x0/gopi/internal/delivery/http/middleware"
)

func RegisterUserRoutes(router *gin.RouterGroup, userHandler *handlers.UserHandler, authMiddleware *middleware.AuthMiddleware) {

	private := router.Group("/user")
	private.Use(authMiddleware.Required())
	{
		private.GET("/profile")
	}
}
