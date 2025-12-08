package middleware

import (
	"github.com/gin-gonic/gin"
	"github.com/motixo/goat-api/internal/delivery/http/response"
	"github.com/motixo/goat-api/internal/domain/entity"
	"github.com/motixo/goat-api/internal/domain/repository"
	"github.com/motixo/goat-api/internal/domain/usecase/permission"
	"github.com/motixo/goat-api/internal/domain/usecase/user"
	"github.com/motixo/goat-api/internal/domain/valueobject"
)

type PermMiddleware struct {
	userUC       user.UseCase
	permissionUS permission.UseCase
	roleCache    repository.RoleRepository
}

func NewPermMiddleware(userUC user.UseCase, permissionUS permission.UseCase, roleCache repository.RoleRepository) *PermMiddleware {
	return &PermMiddleware{
		userUC:       userUC,
		permissionUS: permissionUS,
		roleCache:    roleCache,
	}
}

func (p *PermMiddleware) Require(requiredPerm valueobject.Permission) gin.HandlerFunc {
	return func(c *gin.Context) {
		userIDVal, exists := c.Get(string(UserIDKey))
		if !exists {
			response.Unauthorized(c, "authentication required")
			c.Abort()
			return
		}
		userID, ok := userIDVal.(string)
		if !ok || userID == "" {
			response.Unauthorized(c, "invalid user context")
			c.Abort()
			return
		}

		roleID, err := p.roleCache.GetByUserID(c.Request.Context(), userID)
		if err != nil {
			response.Internal(c)
			c.Abort()
			return
		}
		if roleID == -1 {
			response.Unauthorized(c, "user role not found")
			c.Abort()
			return
		}
		userRole := valueobject.UserRole(roleID)

		perms, err := p.permissionUS.GetPermissionsByRole(c.Request.Context(), userRole)
		if err != nil {
			response.Internal(c)
			c.Abort()
			return
		}

		if !hasPermission(perms, requiredPerm) {
			response.Forbidden(c, "insufficient permissions")
			c.Abort()
			return
		}

		c.Next()
	}
}

func hasPermission(perms []*entity.Permission, required valueobject.Permission) bool {
	requiredStr := string(required)
	fullAccessStr := string(valueobject.PermFullAccess)

	for _, p := range perms {
		if p.Action == requiredStr || p.Action == fullAccessStr {
			return true
		}
	}
	return false
}
