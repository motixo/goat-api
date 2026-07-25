package handlers

import (
	"github.com/gin-gonic/gin"
	httpMiddleware "github.com/motixo/goat-api/internal/delivery/http/middleware"
	"github.com/motixo/goat-api/internal/domain/valueobject"
	"github.com/motixo/goat-api/internal/usecase/authorization"
)

func setHandlerTestPrincipal(
	c *gin.Context,
	userID string,
	sessionID string,
	role valueobject.UserRole,
) {
	permissions, err := valueobject.NewPermissionSet([]valueobject.Permission{
		valueobject.PermFullAccess,
	})
	if err != nil {
		panic(err)
	}
	principal, err := authorization.NewPrincipal(
		userID,
		sessionID,
		7,
		role,
		permissions,
	)
	if err != nil {
		panic(err)
	}
	httpMiddleware.SetPrincipal(c, principal)
}
