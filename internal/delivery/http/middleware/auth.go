package middleware

import (
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/mot0x0/gopi/internal/delivery/http/response"
	"github.com/mot0x0/gopi/internal/domain/usecase/session"
	"github.com/mot0x0/gopi/internal/domain/valueobject"
)

type AuthMiddleware struct {
	sessionUC session.UseCase
	jwtSecret string
}

func NewAuthMiddleware(jwtSecret string, sessionUC session.UseCase) *AuthMiddleware {
	return &AuthMiddleware{
		jwtSecret: jwtSecret,
		sessionUC: sessionUC,
	}
}

func (m *AuthMiddleware) Required() gin.HandlerFunc {
	return func(c *gin.Context) {
		auth := c.GetHeader("Authorization")
		if auth == "" || !strings.HasPrefix(auth, "Bearer ") {
			response.Unauthorized(c, "token required")
			c.Abort()
			return
		}

		token := strings.TrimPrefix(auth, "Bearer ")
		claims, err := valueobject.ParseAndValidate(token, m.jwtSecret) // Use injected secret
		if err != nil || claims.TokenType != valueobject.TokenTypeAccess {
			response.Unauthorized(c, "invalid token")
			c.Abort()
			return
		}

		isValid, err := m.sessionUC.IsJTIValid(c, claims.JTI)
		if err != nil {
			response.Internal(c)
			c.Abort()
			return
		}
		if !isValid {
			response.Unauthorized(c, "invalid token")
			c.Abort()
			return
		}

		c.Set("user_id", claims.UserID)
		c.Next()
	}
}
