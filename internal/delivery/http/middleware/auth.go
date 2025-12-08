package middleware

import (
	"errors"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/motixo/goat-api/internal/delivery/http/response"
	DomainError "github.com/motixo/goat-api/internal/domain/errors"
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
			response.Unauthorized(c, "missing or invalid Authorization header")
			c.Abort()
			return
		}

		token := strings.TrimPrefix(auth, "Bearer ")
		claims, err := m.jwtService.ParseAndValidate(token)
		if err != nil {
			var msg string
			switch {
			case errors.Is(err, DomainError.ErrTokenExpired):
				msg = "token has expired"
			case errors.Is(err, DomainError.ErrUnauthorized):
				msg = "invalid or malformed token"
			default:
				msg = "authentication failed"
			}
			response.Unauthorized(c, msg)
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
			response.Unauthorized(c, "token has been revoked")
			c.Abort()
			return
		}

		c.Set(string(UserIDKey), claims.UserID)
		c.Set(string(SessionIDKey), claims.SessionID)
		c.Next()
	}
}
