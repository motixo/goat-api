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
	private := router.Group(
		"/permission",
		rl.Handler(rlConfig.ProtectedIP),
		authMiddleware.Required(),
		rl.Authenticated(rlConfig.Private),
	)

	classifications.FreshAuthorization(
		private,
		http.MethodGet,
		"/",
		valueobject.PermFullAccess,
		permMiddleware.RequireFreshAuthorization(valueobject.PermFullAccess),
		permissionHandler.GetPermissions,
	)
	classifications.FreshAuthorization(
		private,
		http.MethodGet,
		"/:role",
		valueobject.PermFullAccess,
		permMiddleware.RequireFreshAuthorization(valueobject.PermFullAccess),
		permissionHandler.GetPermissionsByRole,
	)
	classifications.FreshAuthorization(
		private,
		http.MethodPost,
		"/",
		valueobject.PermFullAccess,
		permMiddleware.RequireFreshAuthorization(valueobject.PermFullAccess),
		permissionHandler.CreatePermissin,
	)
	classifications.FreshAuthorization(
		private,
		http.MethodDelete,
		"/:id",
		valueobject.PermFullAccess,
		permMiddleware.RequireFreshAuthorization(valueobject.PermFullAccess),
		permissionHandler.DeletePermissin,
	)
}
