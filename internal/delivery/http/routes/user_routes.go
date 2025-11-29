package routes

import (
	"github.com/gin-gonic/gin"
	"github.com/mot0x0/goth-api/internal/delivery/http/handlers"
	"github.com/mot0x0/goth-api/internal/delivery/http/middleware"
)

func RegisterUserRoutes(router *gin.RouterGroup, userHandler *handlers.UserHandler, sessionHandler *handlers.SessionHandler, authMiddleware *middleware.AuthMiddleware) {

	private := router.Group("/user")
	private.Use(authMiddleware.Required())
	{
		private.GET("/profile")
		private.GET("/sessions", sessionHandler.GetAllUserSessions)
		private.DELETE("/sessions", sessionHandler.DeleteSessions)
	}
}
