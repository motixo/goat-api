package helper

import (
	"github.com/gin-gonic/gin"
	"github.com/motixo/goat-api/internal/domain/service"
)

func LogRequest(logger service.Logger, c *gin.Context) {
	logger.Info("incoming request",
		"endpoint", c.FullPath(),
		"method", c.Request.Method,
		"ip", c.ClientIP(),
		"device", c.GetHeader("User-Agent"),
	)
}
