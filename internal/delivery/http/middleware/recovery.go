package middleware

import (
	"runtime/debug"

	"github.com/gin-gonic/gin"
	"github.com/motixo/goat-api/internal/delivery/http/response"
	"github.com/motixo/goat-api/internal/infrastructure/logger"
)

// Recovery returns a middleware that recovers from panics,
// logs the stack trace and returns a clean 500 error using our standard format.
func Recovery(logger logger.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if r := recover(); r != nil {

				logger.Error("panic occurred", map[string]any{
					"panic": r,
					"stack": string(debug.Stack()),
				})

				response.Internal(c)
				c.Abort()
			}
		}()

		c.Next()
	}
}
