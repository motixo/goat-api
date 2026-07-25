package middleware

import (
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/motixo/goat-api/internal/delivery/http/response"
	domainErrors "github.com/motixo/goat-api/internal/domain/errors"
	"github.com/motixo/goat-api/internal/domain/service"
	"github.com/motixo/goat-api/internal/usecase/authorization"
	"github.com/motixo/goat-api/internal/usecase/session"
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
			response.Unauthorized(c, response.DetailMissingAuthorizationHeader)
			c.Abort()
			return
		}

		token := strings.TrimPrefix(auth, "Bearer ")
		claims, err := m.jwtService.ParseAndValidate(token)
		if err != nil {
			response.WriteProblem(c, response.MapError(err))
			c.Abort()
			return
		}
		if !claims.IsAccess() {
			response.Unauthorized(c, response.DetailAccessTokenRequired)
			c.Abort()
			return
		}

		isValid, err := m.sessionUC.ValidateSession(c.Request.Context(), session.ValidateInput{
			UserID:            claims.UserID,
			SessionID:         claims.SessionID,
			JTI:               claims.JTI,
			CredentialVersion: claims.CredentialVersion,
		})
		if err != nil {
			response.WriteProblem(c, response.MapError(err))
			c.Abort()
			return
		}
		if !isValid {
			response.Unauthorized(c, response.DetailTokenRevoked)
			c.Abort()
			return
		}

		principal, err := authorization.NewPrincipal(
			claims.UserID,
			claims.SessionID,
			claims.CredentialVersion,
			claims.Role,
			claims.Permissions,
		)
		if err != nil {
			response.WriteProblem(c, response.MapError(domainErrors.ErrTokenInvalid))
			c.Abort()
			return
		}

		SetPrincipal(c, principal)
		c.Next()
	}
}
