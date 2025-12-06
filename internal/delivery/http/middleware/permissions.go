package middleware

import (
	"github.com/gin-gonic/gin"
	"github.com/motixo/goat-api/internal/delivery/http/response"
	"github.com/motixo/goat-api/internal/domain/entity"
	"github.com/motixo/goat-api/internal/domain/usecase/permission"
	"github.com/motixo/goat-api/internal/domain/usecase/user"
	"github.com/motixo/goat-api/internal/domain/valueobject"
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
		_, exists := c.Get("user_role")
		if !exists {
			response.Unauthorized(c, "Missing or invalid authentication token.")
			return
		}

		userRole := valueobject.UserRole(c.GetInt8("user_role"))
		perms, err := p.permissionUS.GetPermissionsByRole(c.Request.Context(), userRole)
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
