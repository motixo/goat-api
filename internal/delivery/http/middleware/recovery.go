// internal/delivery/http/middleware/recovery.go
package middleware

import (
	"log"
	"runtime/debug"

	"github.com/gin-gonic/gin"
	"github.com/mot0x0/gopi/internal/delivery/http/response"
)

// Recovery returns a middleware that recovers from panics,
// logs the stack trace and returns a clean 500 error using our standard format.
func Recovery() gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if r := recover(); r != nil {
				log.Printf("[PANIC] %v\n%s", r, debug.Stack())
				response.Internal(c)
				c.Abort()
			}
		}()

		c.Next()
	}
}
