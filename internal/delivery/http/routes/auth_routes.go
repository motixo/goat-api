package routes

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/motixo/goat-api/internal/delivery/http/handlers"
	"github.com/motixo/goat-api/internal/delivery/http/middleware"
)

func RegisterAuthRoutes(
	router *gin.RouterGroup,
	authHandler *handlers.AuthHandler,
	authMiddleware *middleware.AuthMiddleware,
	permMiddleware *middleware.PermMiddleware,
	rl *middleware.RateLimitMiddleware,
	rlConfig middleware.RateLimitConfig,
	classifications *ClassificationRegistry,
) {
	public := router.Group("/auth")
	{
		classifications.Public(
			public,
			http.MethodPost,
			"/login",
			rl.Handler(rlConfig.Auth),
			authHandler.Login,
		)
		classifications.Public(
			public,
			http.MethodPost,
			"/signup",
			rl.Handler(rlConfig.Auth),
			authHandler.Register,
		)
		classifications.Public(
			public,
			http.MethodPost,
			"/refresh",
			rl.Handler(rlConfig.Private),
			authHandler.Refresh,
		)
	}

	private := router.Group(
		"/auth",
		rl.Handler(rlConfig.ProtectedIP),
		authMiddleware.Required(),
		rl.Authenticated(rlConfig.Private),
	)
	{
		classifications.FreshIdentity(
			private,
			http.MethodPost,
			"/logout",
			permMiddleware.FreshIdentity(),
			authHandler.Logout,
		)
	}
}
