package helper

import (
	"github.com/gin-gonic/gin"
	"github.com/motixo/goat-api/internal/infrastructure/logger"
)

func LogRequest(logger logger.Logger, c *gin.Context) {
	logger.Info("incoming request",
		"endpoint", c.FullPath(),
		"method", c.Request.Method,
		"ip", c.ClientIP(),
		"device", c.GetHeader("User-Agent"),
	)
}
