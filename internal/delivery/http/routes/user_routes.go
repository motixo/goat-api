package routes

import (
	"github.com/gin-gonic/gin"
	"github.com/motixo/goth-api/internal/delivery/http/handlers"
	"github.com/motixo/goth-api/internal/delivery/http/middleware"
	"github.com/motixo/goth-api/internal/domain/valueobject"
)

func RegisterUserRoutes(
	router *gin.RouterGroup,
	userHandler *handlers.UserHandler,
	sessionHandler *handlers.SessionHandler,
	authMiddleware *middleware.AuthMiddleware,
	permMiddleware *middleware.PermMiddleware,
) {

	private := router.Group("/user")
	private.Use(authMiddleware.Required())
	{
		private.GET("/profile")
		private.GET("/sessions",
			permMiddleware.Require(valueobject.PermSessionRead),
			sessionHandler.GetAllUserSessions,
		)
		private.DELETE("/sessions", sessionHandler.DeleteSessions)
	}
}
