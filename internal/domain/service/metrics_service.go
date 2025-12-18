package service

import "github.com/prometheus/client_golang/prometheus"

type MetricsService interface {
	// HTTP metrics
	RecordHTTPRequest(duration float64, method, path, statusCode string)
	RecordHTTPActiveRequests(increment bool)

	// Database metrics
	RecordDBQuery(duration float64, operation, success string)

	// Cache metrics
	RecordCacheHit(hit bool, cacheType string)

	// Business metrics
	RecordUserLogin(success bool)
	RecordTokenRefresh(success bool)

	// Get registry for HTTP handler
	GetRegistry() *prometheus.Registry
}
