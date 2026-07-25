package routes

import (
	"net/http"

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
	rl *middleware.RateLimitMiddleware,
	rlConfig middleware.RateLimitConfig,
	classifications *ClassificationRegistry,
) {
	private := router.Group("/user")
	privateRateLimit := rl.Handler(rlConfig.Private)

	classifications.FreshAuthorization(
		private,
		http.MethodPost,
		"/",
		valueobject.PermFullAccess,
		authMiddleware.Required(),
		privateRateLimit,
		permMiddleware.RequireFreshAuthorization(valueobject.PermFullAccess),
		userHandler.CreateUser,
	)
	classifications.Snapshot(
		private,
		http.MethodGet,
		"/",
		"",
		authMiddleware.Required(),
		privateRateLimit,
		userHandler.GetUser,
	)
	classifications.FreshAuthorization(
		private,
		http.MethodGet,
		"/:id",
		valueobject.PermUserRead,
		authMiddleware.Required(),
		privateRateLimit,
		permMiddleware.RequireFreshAuthorization(valueobject.PermUserRead),
		userHandler.GetUser,
	)
	classifications.FreshAuthorization(
		private,
		http.MethodGet,
		"/list",
		valueobject.PermUserRead,
		authMiddleware.Required(),
		privateRateLimit,
		permMiddleware.RequireFreshAuthorization(valueobject.PermUserRead),
		userHandler.GetUserList,
	)
	classifications.FreshIdentity(
		private,
		http.MethodDelete,
		"/",
		authMiddleware.Required(),
		privateRateLimit,
		permMiddleware.FreshIdentity(),
		userHandler.DeleteUser,
	)
	classifications.FreshAuthorization(
		private,
		http.MethodDelete,
		"/:id",
		valueobject.PermUserDelete,
		authMiddleware.Required(),
		privateRateLimit,
		permMiddleware.RequireFreshAuthorization(valueobject.PermUserDelete),
		userHandler.DeleteUser,
	)
	classifications.FreshAuthorization(
		private,
		http.MethodPut,
		"/:id",
		valueobject.PermFullAccess,
		authMiddleware.Required(),
		privateRateLimit,
		permMiddleware.RequireFreshAuthorization(valueobject.PermFullAccess),
		userHandler.UpdateUser,
	)
	classifications.FreshIdentity(
		private,
		http.MethodPatch,
		"/change-email",
		authMiddleware.Required(),
		privateRateLimit,
		permMiddleware.FreshIdentity(),
		userHandler.ChangeEmail,
	)
	classifications.FreshIdentity(
		private,
		http.MethodPatch,
		"/change-password",
		authMiddleware.Required(),
		privateRateLimit,
		permMiddleware.FreshIdentity(),
		userHandler.ChangePassword,
	)
	classifications.FreshAuthorization(
		private,
		http.MethodPatch,
		"/:id/change-role",
		valueobject.PermUserChangeRole,
		authMiddleware.Required(),
		privateRateLimit,
		permMiddleware.RequireFreshAuthorization(valueobject.PermUserChangeRole),
		userHandler.ChangeRole,
	)
	classifications.FreshAuthorization(
		private,
		http.MethodPatch,
		"/:id/change-status",
		valueobject.PermUserChangeStatus,
		authMiddleware.Required(),
		privateRateLimit,
		permMiddleware.RequireFreshAuthorization(valueobject.PermUserChangeStatus),
		userHandler.ChangeStatus,
	)
	classifications.FreshIdentity(
		private,
		http.MethodGet,
		"/sessions",
		authMiddleware.Required(),
		privateRateLimit,
		permMiddleware.FreshIdentity(),
		sessionHandler.GetAllUserSessions,
	)
	classifications.FreshIdentity(
		private,
		http.MethodDelete,
		"/sessions",
		authMiddleware.Required(),
		privateRateLimit,
		permMiddleware.FreshIdentity(),
		sessionHandler.DeleteSessions,
	)
}
