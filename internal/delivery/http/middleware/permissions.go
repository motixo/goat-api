package middleware

import (
	"github.com/gin-gonic/gin"
	"github.com/motixo/goth-api/internal/delivery/http/response"
	"github.com/motixo/goth-api/internal/domain/entity"
	"github.com/motixo/goth-api/internal/domain/usecase/permission"
	"github.com/motixo/goth-api/internal/domain/usecase/user"
	"github.com/motixo/goth-api/internal/domain/valueobject"
)

type PermMiddleware struct {
	userUC       user.UseCase
	permissionUS permission.UseCase
}

func NewPermMiddleware(userUC user.UseCase, permissionUS permission.UseCase) *PermMiddleware {
	return &PermMiddleware{
		userUC:       userUC,
		permissionUS: permissionUS,
	}
}

func (p *PermMiddleware) Require(permValue valueobject.Permission) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID := c.GetString("user_id")
		if userID == "" {
			response.Unauthorized(c, "Missing or invalid authentication token.")
			return
		}

		user, err := p.userUC.GetUser(c.Request.Context(), userID)
		if err != nil {
			response.DomainError(c, err)
			return
		}

		perms, err := p.permissionUS.GetPermissionsByRole(c.Request.Context(), user.Role)
		if err != nil {
			response.Internal(c)
			return
		}

		if !hasPermission(perms, permValue) {
			response.Forbidden(c, "You do not have permission to perform this action.")
			return
		}

		c.Next()
	}
}

func hasPermission(perms *[]entity.Permission, required valueobject.Permission) bool {
	if perms == nil {
		return false
	}

	for _, p := range *perms {
		if p.Action == string(required) || p.Action == string(valueobject.PermFullAccess) {
			return true
		}
	}
	return false
}
