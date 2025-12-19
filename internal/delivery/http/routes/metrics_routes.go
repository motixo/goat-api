package routes

import (
	"github.com/gin-gonic/gin"
	"github.com/motixo/goat-api/internal/domain/service"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func RegisterMetricsRoutes(r *gin.RouterGroup, metricsService service.MetricsService) {
	r.GET("/metrics", gin.WrapH(promhttp.HandlerFor(
		metricsService.GetRegistry(),
		promhttp.HandlerOpts{EnableOpenMetrics: true},
	)))
}
