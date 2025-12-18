package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

type PrometheusMetrics struct {
	registry *prometheus.Registry

	// HTTP metrics
	httpRequestDuration *prometheus.HistogramVec
	httpActiveRequests  prometheus.Gauge
	httpRequestsTotal   *prometheus.CounterVec

	// Database metrics
	dbQueryDuration *prometheus.HistogramVec
	dbQueriesTotal  *prometheus.CounterVec

	// Cache metrics
	cacheHits *prometheus.CounterVec

	// Business metrics
	userLoginsTotal   *prometheus.CounterVec
	tokenRefreshTotal *prometheus.CounterVec
}

func NewPrometheusMetrics() *PrometheusMetrics {
	registry := prometheus.NewRegistry()

	// Register default Go metrics
	registry.MustRegister(collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}))
	registry.MustRegister(collectors.NewGoCollector())

	m := &PrometheusMetrics{
		registry: registry,

		// HTTP metrics
		httpRequestDuration: promauto.With(registry).NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "http_request_duration_seconds",
				Help:    "HTTP request duration in seconds",
				Buckets: prometheus.DefBuckets,
			},
			[]string{"method", "path", "status_code"},
		),
		httpActiveRequests: promauto.With(registry).NewGauge(
			prometheus.GaugeOpts{
				Name: "http_active_requests",
				Help: "Number of active HTTP requests",
			},
		),
		httpRequestsTotal: promauto.With(registry).NewCounterVec(
			prometheus.CounterOpts{
				Name: "http_requests_total",
				Help: "Total number of HTTP requests",
			},
			[]string{"method", "path", "status_code"},
		),

		// Database metrics
		dbQueryDuration: promauto.With(registry).NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "db_query_duration_seconds",
				Help:    "Database query duration in seconds",
				Buckets: []float64{0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5},
			},
			[]string{"operation", "success"},
		),
		dbQueriesTotal: promauto.With(registry).NewCounterVec(
			prometheus.CounterOpts{
				Name: "db_queries_total",
				Help: "Total number of database queries",
			},
			[]string{"operation", "success"},
		),

		// Cache metrics
		cacheHits: promauto.With(registry).NewCounterVec(
			prometheus.CounterOpts{
				Name: "cache_hits_total",
				Help: "Total cache hits/misses",
			},
			[]string{"cache_type", "hit"},
		),

		// Business metrics
		userLoginsTotal: promauto.With(registry).NewCounterVec(
			prometheus.CounterOpts{
				Name: "user_logins_total",
				Help: "Total user login attempts",
			},
			[]string{"success"},
		),
		tokenRefreshTotal: promauto.With(registry).NewCounterVec(
			prometheus.CounterOpts{
				Name: "token_refresh_total",
				Help: "Total token refresh attempts",
			},
			[]string{"success"},
		),
	}

	return m
}

// HTTP metrics
func (m *PrometheusMetrics) RecordHTTPRequest(duration float64, method, path, statusCode string) {
	m.httpRequestDuration.WithLabelValues(method, path, statusCode).Observe(duration)
	m.httpRequestsTotal.WithLabelValues(method, path, statusCode).Inc()
}

func (m *PrometheusMetrics) RecordHTTPActiveRequests(increment bool) {
	if increment {
		m.httpActiveRequests.Inc()
	} else {
		m.httpActiveRequests.Dec()
	}
}

// Database metrics
func (m *PrometheusMetrics) RecordDBQuery(duration float64, operation, success string) {
	m.dbQueryDuration.WithLabelValues(operation, success).Observe(duration)
	m.dbQueriesTotal.WithLabelValues(operation, success).Inc()
}

// Cache metrics
func (m *PrometheusMetrics) RecordCacheHit(hit bool, cacheType string) {
	hitStr := "true"
	if !hit {
		hitStr = "false"
	}
	m.cacheHits.WithLabelValues(cacheType, hitStr).Inc()
}

// Business metrics
func (m *PrometheusMetrics) RecordUserLogin(success bool) {
	successStr := "true"
	if !success {
		successStr = "false"
	}
	m.userLoginsTotal.WithLabelValues(successStr).Inc()
}

func (m *PrometheusMetrics) RecordTokenRefresh(success bool) {
	successStr := "true"
	if !success {
		successStr = "false"
	}
	m.tokenRefreshTotal.WithLabelValues(successStr).Inc()
}

func (m *PrometheusMetrics) GetRegistry() *prometheus.Registry {
	return m.registry
}
