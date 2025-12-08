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

func (p *PermMiddleware) Require(requiredPerm valueobject.Permission) gin.HandlerFunc {
	return func(c *gin.Context) {
		// 1. Validate auth context (must come from AuthMiddleware)
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

		// 2. Safely extract user role (Gin stores numbers as float64!)
		userRoleVal, exists := c.Get(string(UserRoleKey))
		if !exists {
			response.Unauthorized(c, "missing role in context")
			c.Abort()
			return
		}
		userRoleFloat, ok := userRoleVal.(float64)
		if !ok {
			response.Internal(c)
			c.Abort()
			return
		}
		userRole := valueobject.UserRole(int8(userRoleFloat))

		// 3. Fetch permissions (with caching!)
		perms, err := p.permissionUS.GetPermissionsByRole(c.Request.Context(), userRole)
		if err != nil {
			response.Internal(c)
			c.Abort()
			return
		}

		// 4. Check permission
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
