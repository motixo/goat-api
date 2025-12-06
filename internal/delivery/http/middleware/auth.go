package middleware

import (
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/motixo/goat-api/internal/delivery/http/response"
	"github.com/motixo/goat-api/internal/domain/service"
	"github.com/motixo/goat-api/internal/domain/usecase/session"
)

type AuthMiddleware struct {
	sessionUC  session.UseCase
	jwtService service.JWTService
}

func NewAuthMiddleware(jwtService service.JWTService, sessionUC session.UseCase) *AuthMiddleware {
	return &AuthMiddleware{
		jwtService: jwtService,
		sessionUC:  sessionUC,
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
		claims, err := m.jwtService.ParseAndValidate(token)
		if err != nil {
			response.Unauthorized(c, "invalid token")
			c.Abort()
			return
		}

		if err := m.jwtService.ValidateClaims(claims); err != nil {
			response.Unauthorized(c, "token validation failed")
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
		c.Set("session_id", claims.SessionID)
		c.Set("user_role", claims.UserRole)
		c.Next()
	}
}
