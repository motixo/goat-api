package routes

import (
	"github.com/gin-gonic/gin"
	"github.com/mot0x0/gopi/internal/delivery/http/handlers"
	"github.com/mot0x0/gopi/internal/delivery/http/middleware"
)

func RegisterUserRoutes(router *gin.RouterGroup, userHandler *handlers.UserHandler) {
	public := router.Group("/signup")
	public.Use()
	{
		public.POST("", userHandler.Register)
	}
	private := router.Group("/user")
	private.Use(middleware.AuthRequired())
	{
		private.GET("/profile")
	}
}
