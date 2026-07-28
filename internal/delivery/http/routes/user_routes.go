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
	private := router.Group(
		"/user",
		rl.Handler(rlConfig.ProtectedIP),
		authMiddleware.Required(),
		rl.Authenticated(rlConfig.Private),
	)

	classifications.FreshAuthorization(
		private,
		http.MethodPost,
		"/",
		valueobject.PermFullAccess,
		permMiddleware.RequireFreshAuthorization(valueobject.PermFullAccess),
		userHandler.CreateUser,
	)
	classifications.Snapshot(
		private,
		http.MethodGet,
		"/",
		"",
		userHandler.GetUser,
	)
	classifications.FreshAuthorization(
		private,
		http.MethodGet,
		"/:id",
		valueobject.PermUserRead,
		permMiddleware.RequireFreshAuthorization(valueobject.PermUserRead),
		userHandler.GetUser,
	)
	classifications.FreshAuthorization(
		private,
		http.MethodGet,
		"/list",
		valueobject.PermUserRead,
		permMiddleware.RequireFreshAuthorization(valueobject.PermUserRead),
		userHandler.GetUserList,
	)
	classifications.FreshIdentity(
		private,
		http.MethodDelete,
		"/",
		permMiddleware.FreshIdentity(),
		userHandler.DeleteUser,
	)
	classifications.FreshAuthorization(
		private,
		http.MethodDelete,
		"/:id",
		valueobject.PermUserDelete,
		permMiddleware.RequireFreshAuthorization(valueobject.PermUserDelete),
		userHandler.DeleteUser,
	)
	classifications.FreshAuthorization(
		private,
		http.MethodPut,
		"/:id",
		valueobject.PermFullAccess,
		permMiddleware.RequireFreshAuthorization(valueobject.PermFullAccess),
		userHandler.UpdateUser,
	)
	classifications.FreshIdentity(
		private,
		http.MethodPatch,
		"/change-email",
		permMiddleware.FreshIdentity(),
		userHandler.ChangeEmail,
	)
	classifications.FreshIdentity(
		private,
		http.MethodPatch,
		"/change-password",
		permMiddleware.FreshIdentity(),
		userHandler.ChangePassword,
	)
	classifications.FreshAuthorization(
		private,
		http.MethodPatch,
		"/:id/change-role",
		valueobject.PermUserChangeRole,
		permMiddleware.RequireFreshAuthorization(valueobject.PermUserChangeRole),
		userHandler.ChangeRole,
	)
	classifications.FreshAuthorization(
		private,
		http.MethodPatch,
		"/:id/change-status",
		valueobject.PermUserChangeStatus,
		permMiddleware.RequireFreshAuthorization(valueobject.PermUserChangeStatus),
		userHandler.ChangeStatus,
	)
	classifications.FreshIdentity(
		private,
		http.MethodGet,
		"/sessions",
		permMiddleware.FreshIdentity(),
		sessionHandler.GetAllUserSessions,
	)
	classifications.FreshIdentity(
		private,
		http.MethodDelete,
		"/sessions",
		permMiddleware.FreshIdentity(),
		sessionHandler.DeleteSessions,
	)
}
