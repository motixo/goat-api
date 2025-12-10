package routes

import (
	"github.com/gin-gonic/gin"
	"github.com/motixo/goat-api/internal/delivery/http/handlers"
	"github.com/motixo/goat-api/internal/delivery/http/middleware"
	"github.com/motixo/goat-api/internal/domain/valueobject"
)

func RegisterPermissionRoutes(
	router *gin.RouterGroup,
	permissionHandler *handlers.PermissionHandler,
	authMiddleware *middleware.AuthMiddleware,
	permMiddleware *middleware.PermMiddleware,
) {
	private := router.Group("/permission")
	private.Use(authMiddleware.Required())
	private.Use(permMiddleware.Require(valueobject.PermFullAccess))
	{
		private.GET("/",
			permissionHandler.GetPermissions)
		private.GET("/:role",
			permissionHandler.GetPermissionsByRole)
		private.POST("/",
			permissionHandler.CreatePermissin)
		private.DELETE("/:id",
			permissionHandler.DeletePermissin)

	}

}
