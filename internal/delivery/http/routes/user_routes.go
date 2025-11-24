// delivery/http/routes/user_routes.go
package routes

import (
	"github.com/gin-gonic/gin"
	"github.com/mot0x0/gopi/internal/delivery/http/handlers"
)

func RegisterUserRoutes(router *gin.RouterGroup, userHandler *handlers.UserHandler) {
	users := router.Group("/users")
	{
		users.POST("/register", userHandler.Register)
		users.POST("/login", userHandler.Login)

		// TODO
		// auth := users.Group("")
		// auth.Use(AuthMiddleware())
		// {
		//     auth.GET("/profile", userHandler.GetProfile)
		//     auth.PUT("/profile", userHandler.UpdateProfile)
		// }
	}
}
