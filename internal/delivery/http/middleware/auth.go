package middleware

import (
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/motixo/goat-api/internal/delivery/http/response"
	"github.com/motixo/goat-api/internal/domain/service"
	"github.com/motixo/goat-api/internal/domain/valueobject"
	"github.com/motixo/goat-api/internal/usecase/session"
)

type AuthMiddleware struct {
	sessionUC  session.UseCase
	jwtService service.JWTService
	userCache  service.UserCacheService
}

func NewAuthMiddleware(jwtService service.JWTService, sessionUC session.UseCase, userCache service.UserCacheService) *AuthMiddleware {
	return &AuthMiddleware{
		jwtService: jwtService,
		sessionUC:  sessionUC,
		userCache:  userCache,
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

		isValid, err := m.sessionUC.IsJTIValid(c, claims.JTI)
		if err != nil {
			response.Internal(c)
			c.Abort()
			return
		}
		if !isValid {
			response.Unauthorized(c, response.DetailTokenRevoked)
			c.Abort()
			return
		}

		userStatus, err := m.userCache.GetUserStatus(c.Request.Context(), claims.UserID)
		if err != nil {
			response.Internal(c)
			c.Abort()
			return
		}

		switch userStatus {
		case valueobject.StatusInactive:
			response.Unauthorized(c, response.DetailAccountNotActivated)
			c.Abort()
			return
		case valueobject.StatusSuspended:
			response.Unauthorized(c, response.DetailAccountSuspended)
			c.Abort()
			return
		case valueobject.StatusUnknown:
			response.Unauthorized(c, response.DetailContactSupport)
			c.Abort()
			return
		}

		c.Set(string(UserIDKey), claims.UserID)
		c.Set(string(SessionIDKey), claims.SessionID)
		c.Next()
	}
}
