package middleware

import (
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/mot0x0/gopi/internal/config"
	"github.com/mot0x0/gopi/internal/delivery/http/response"
	"github.com/mot0x0/gopi/internal/domain/valueobject"
)

func AuthRequired() gin.HandlerFunc {
	return func(c *gin.Context) {
		auth := c.GetHeader("Authorization")
		if auth == "" || !strings.HasPrefix(auth, "Bearer ") {
			response.Unauthorized(c, "token required")
			c.Abort()
			return
		}

		token := strings.TrimPrefix(auth, "Bearer ")
		claims, err := valueobject.ParseAndValidate(token, config.Get().JWTSecret)
		if err != nil || claims.TokenType != valueobject.TokenTypeAccess {
			response.Unauthorized(c, "invalid token")
			c.Abort()
			return
		}

		c.Set("user_id", claims.UserID)
		c.Set("user_email", claims.Email)
		c.Next()
	}
}
