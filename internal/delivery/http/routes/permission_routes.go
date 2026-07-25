package routes

import (
	"net/http"

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
	rl *middleware.RateLimitMiddleware,
	rlConfig middleware.RateLimitConfig,
	classifications *ClassificationRegistry,
) {
	private := router.Group("/permission")
	privateRateLimit := rl.Handler(rlConfig.Private)

	classifications.FreshAuthorization(
		private,
		http.MethodGet,
		"/",
		valueobject.PermFullAccess,
		authMiddleware.Required(),
		permMiddleware.RequireFreshAuthorization(valueobject.PermFullAccess),
		privateRateLimit,
		permissionHandler.GetPermissions,
	)
	classifications.FreshAuthorization(
		private,
		http.MethodGet,
		"/:role",
		valueobject.PermFullAccess,
		authMiddleware.Required(),
		permMiddleware.RequireFreshAuthorization(valueobject.PermFullAccess),
		privateRateLimit,
		permissionHandler.GetPermissionsByRole,
	)
	classifications.FreshAuthorization(
		private,
		http.MethodPost,
		"/",
		valueobject.PermFullAccess,
		authMiddleware.Required(),
		permMiddleware.RequireFreshAuthorization(valueobject.PermFullAccess),
		privateRateLimit,
		permissionHandler.CreatePermissin,
	)
	classifications.FreshAuthorization(
		private,
		http.MethodDelete,
		"/:id",
		valueobject.PermFullAccess,
		authMiddleware.Required(),
		permMiddleware.RequireFreshAuthorization(valueobject.PermFullAccess),
		privateRateLimit,
		permissionHandler.DeletePermissin,
	)
}
