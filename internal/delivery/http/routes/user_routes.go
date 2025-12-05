package routes

import (
	"github.com/gin-gonic/gin"
	"github.com/motixo/goat-api/internal/delivery/http/handlers"
	"github.com/motixo/goat-api/internal/delivery/http/middleware"
	"github.com/motixo/goat-api/internal/domain/valueobject"
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
		private.GET("/profile",
			permMiddleware.Require(valueobject.PermUserRead),
		)
		private.GET("/sessions",
			permMiddleware.Require(valueobject.PermSessionRead),
			sessionHandler.GetAllUserSessions,
		)
		private.DELETE("/sessions",
			permMiddleware.Require(valueobject.PermSessionDelete),
			sessionHandler.DeleteSessions)
	}
}
