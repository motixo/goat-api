package middleware

import (
	"github.com/gin-gonic/gin"
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
		user, err := p.userUC.GetUser(c.Request.Context(), userID)
		if err != nil {
			c.AbortWithStatusJSON(404, gin.H{"error": "user not found"})
			return
		}
		permissions, err := p.permissionUS.GetPermissionsByRole(c.Request.Context(), user.Role)
		if err != nil {
			c.AbortWithStatusJSON(500, gin.H{"error": "cannot load permissions"})
			return
		}
		allowed := false
		for _, perm := range *permissions {
			if perm.Action == string(permValue) {
				allowed = true
				break
			}
		}
		if !allowed {
			c.AbortWithStatusJSON(403, gin.H{"error": "permission denied"})
			return
		}
		c.Next()
	}
}
