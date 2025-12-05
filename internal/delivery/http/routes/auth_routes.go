package routes

import (
	"github.com/gin-gonic/gin"
	"github.com/motixo/goat-api/internal/delivery/http/handlers"
	"github.com/motixo/goat-api/internal/delivery/http/middleware"
	"github.com/motixo/goat-api/internal/domain/valueobject"
)

func RegisterAuthRoutes(
	router *gin.RouterGroup,
	authHandler *handlers.AuthHandler,
	authMiddleware *middleware.AuthMiddleware,
	permMiddleware *middleware.PermMiddleware,
) {
	public := router.Group("/auth")
	{
		public.POST("/login", authHandler.Login)
		public.POST("/signup", authHandler.Register)
		public.POST("/refresh", authHandler.Refresh)
	}

	private := router.Group("/auth")
	private.Use(authMiddleware.Required())
	{
		private.POST("/logout",
			permMiddleware.Require(valueobject.PermSessionDelete),
			authHandler.Logout)
	}

}
