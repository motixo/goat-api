package middleware

import (
	"github.com/gin-gonic/gin"
	"github.com/motixo/goat-api/internal/delivery/http/response"
	"github.com/motixo/goat-api/internal/domain/valueobject"
	"github.com/motixo/goat-api/internal/usecase/authorization"
)

type PermMiddleware struct {
	authorization authorization.UseCase
}

func NewPermMiddleware(usecase authorization.UseCase) *PermMiddleware {
	return &PermMiddleware{authorization: usecase}
}

func (m *PermMiddleware) RequireSnapshot(
	required valueobject.Permission,
) gin.HandlerFunc {
	return func(c *gin.Context) {
		principal, ok := PrincipalFrom(c)
		if !ok {
			response.Unauthorized(c, response.DetailAuthenticationRequired)
			c.Abort()
			return
		}
		if !principal.Permissions().Has(required) {
			response.WriteProblem(c, response.MapError(authorization.ErrPermissionDenied))
			c.Abort()
			return
		}
		c.Next()
	}
}

func (m *PermMiddleware) FreshIdentity() gin.HandlerFunc {
	return func(c *gin.Context) {
		principal, ok := PrincipalFrom(c)
		if !ok {
			response.Unauthorized(c, response.DetailAuthenticationRequired)
			c.Abort()
			return
		}
		fresh, err := m.authorization.AuthorizeFreshIdentity(
			c.Request.Context(),
			principal,
		)
		if err != nil {
			response.WriteProblem(c, response.MapError(err))
			c.Abort()
			return
		}
		SetPrincipal(c, fresh)
		c.Next()
	}
}

func (m *PermMiddleware) RequireFreshAuthorization(
	required valueobject.Permission,
) gin.HandlerFunc {
	return func(c *gin.Context) {
		principal, ok := PrincipalFrom(c)
		if !ok {
			response.Unauthorized(c, response.DetailAuthenticationRequired)
			c.Abort()
			return
		}
		fresh, err := m.authorization.AuthorizeFresh(
			c.Request.Context(),
			principal,
			required,
		)
		if err != nil {
			response.WriteProblem(c, response.MapError(err))
			c.Abort()
			return
		}
		SetPrincipal(c, fresh)
		c.Next()
	}
}
