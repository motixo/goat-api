package middleware

import (
	"github.com/gin-gonic/gin"
	"github.com/motixo/goat-api/internal/usecase/authorization"
)

type ContextKey string

const principalKey ContextKey = "verified_principal"

func SetPrincipal(c *gin.Context, principal authorization.Principal) {
	c.Set(string(principalKey), principal)
}

func PrincipalFrom(c *gin.Context) (authorization.Principal, bool) {
	value, exists := c.Get(string(principalKey))
	if !exists {
		return authorization.Principal{}, false
	}
	principal, ok := value.(authorization.Principal)
	return principal, ok && principal.IsValid()
}
