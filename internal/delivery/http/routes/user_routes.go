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

		private.POST("/",
			permMiddleware.Require(valueobject.PermFullAccess),
			userHandler.CreateUser,
		)

		private.GET("/", userHandler.GetUser)

		private.GET("/:id",
			permMiddleware.Require(valueobject.PermUserRead),
			userHandler.GetUser,
		)

		private.GET("/list",
			permMiddleware.Require(valueobject.PermUserRead),
			userHandler.GetUserList,
		)

		private.DELETE("/delete", userHandler.DeleteUser)

		private.DELETE("/delete/:id",
			permMiddleware.Require(valueobject.PermUserDelete),
			userHandler.DeleteUser,
		)

		private.PUT("/change-email", userHandler.ChangeEmail)

		private.PUT("/change-password", userHandler.ChangePassword)

		private.PUT("/change-role",
			permMiddleware.Require(valueobject.PermUserChangeRole),
			userHandler.ChangeRole,
		)

		private.PUT("/change-status",
			permMiddleware.Require(valueobject.PermUserChangeStatus),
			userHandler.ChangeStatus,
		)

		private.GET("/sessions",
			sessionHandler.GetAllUserSessions,
		)
		private.DELETE("/sessions",
			sessionHandler.DeleteSessions)
	}
}
