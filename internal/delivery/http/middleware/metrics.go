package middleware

import (
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/motixo/goat-api/internal/domain/service"
)

type MetricsMiddleware struct {
	metrics service.MetricsService
}

func NewMetricsMiddleware(metrics service.MetricsService) *MetricsMiddleware {
	return &MetricsMiddleware{metrics: metrics}
}

func (m *MetricsMiddleware) Handler() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()

		m.metrics.RecordHTTPActiveRequests(true)
		c.Next()

		m.metrics.RecordHTTPActiveRequests(false)

		duration := time.Since(start).Seconds()
		method := c.Request.Method
		path := c.FullPath()
		if path == "" {
			path = c.Request.URL.Path
		}
		statusCode := strconv.Itoa(c.Writer.Status())

		m.metrics.RecordHTTPRequest(duration, method, path, statusCode)
	}
}
