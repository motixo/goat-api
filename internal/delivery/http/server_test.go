package http

import (
	"context"
	"errors"
	"io"
	"net"
	netHTTP "net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/motixo/goat-api/internal/delivery/http/middleware"
	"github.com/motixo/goat-api/internal/delivery/http/routes"
	"github.com/motixo/goat-api/internal/domain/valueobject"
	"github.com/prometheus/client_golang/prometheus"
)

func TestServerStartReturnsListenerFailureSynchronously(t *testing.T) {
	t.Parallel()

	listenErr := errors.New("address already in use")
	server := &Server{
		httpServer: &netHTTP.Server{Handler: netHTTP.NewServeMux()},
		listen: func(string, string) (net.Listener, error) {
			return nil, listenErr
		},
	}
	serveErrors, err := server.Start("127.0.0.1:8080")
	if !errors.Is(err, listenErr) {
		t.Fatalf("Start() error = %v, want wrapped listener failure", err)
	}
	if serveErrors != nil {
		t.Fatal("Start() returned a serve channel after listener failure")
	}
}

func TestServerStartIsReadyForImmediateShutdown(t *testing.T) {
	t.Parallel()

	listener := newBlockingListener()
	server := &Server{
		httpServer: &netHTTP.Server{Handler: netHTTP.NewServeMux()},
		listen: func(string, string) (net.Listener, error) {
			return listener, nil
		},
	}
	serveErrors, err := server.Start("127.0.0.1:0")
	if err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	if _, err := server.Start("127.0.0.1:0"); !errors.Is(err, ErrServerAlreadyStarted) {
		t.Fatalf("second Start() error = %v, want ErrServerAlreadyStarted", err)
	}
	if !server.accepting.Load() {
		t.Fatal("server was not marked ready after its listener began accepting")
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := server.Shutdown(ctx); err != nil {
		t.Fatalf("Shutdown() error = %v", err)
	}
	if server.accepting.Load() {
		t.Fatal("server remained ready after shutdown began")
	}
	select {
	case serveErr := <-serveErrors:
		if serveErr != nil {
			t.Fatalf("serve error = %v", serveErr)
		}
	case <-time.After(time.Second):
		t.Fatal("HTTP serve loop did not stop")
	}
	if err := server.Close(); err != nil {
		t.Fatalf("Close() after Shutdown error = %v", err)
	}
}

func TestEveryRegisteredRouteHasExplicitAuthorizationClassification(t *testing.T) {
	server, err := NewServer(
		testServerConfig(GinModeRelease),
		ServerDependencies{
			MetricsService:   routeClassificationMetrics{registry: prometheus.NewRegistry()},
			ReadinessChecker: readinessCheckFunc(func(context.Context) error { return nil }),
		},
		middleware.RateLimitConfig{},
	)
	if err != nil {
		t.Fatalf("NewServer() error = %v", err)
	}

	// Keep this expected inventory explicit: a new route must receive a
	// repository-specific risk classification before this test will pass.
	expected := map[string]routes.RouteClassification{
		"GET /api/health":                      routeClass("GET", "/api/health", routes.AuthorizationPublic, ""),
		"GET /api/ready":                       routeClass("GET", "/api/ready", routes.AuthorizationPublic, ""),
		"GET /api/metrics":                     routeClass("GET", "/api/metrics", routes.AuthorizationPublic, ""),
		"POST /api/v1/auth/login":              routeClass("POST", "/api/v1/auth/login", routes.AuthorizationPublic, ""),
		"POST /api/v1/auth/signup":             routeClass("POST", "/api/v1/auth/signup", routes.AuthorizationPublic, ""),
		"POST /api/v1/auth/refresh":            routeClass("POST", "/api/v1/auth/refresh", routes.AuthorizationPublic, ""),
		"POST /api/v1/auth/logout":             routeClass("POST", "/api/v1/auth/logout", routes.AuthorizationFreshIdentity, ""),
		"POST /api/v1/user/":                   routeClass("POST", "/api/v1/user/", routes.AuthorizationFreshAuthorization, valueobject.PermFullAccess),
		"GET /api/v1/user/":                    routeClass("GET", "/api/v1/user/", routes.AuthorizationSnapshot, ""),
		"GET /api/v1/user/:id":                 routeClass("GET", "/api/v1/user/:id", routes.AuthorizationFreshAuthorization, valueobject.PermUserRead),
		"GET /api/v1/user/list":                routeClass("GET", "/api/v1/user/list", routes.AuthorizationFreshAuthorization, valueobject.PermUserRead),
		"DELETE /api/v1/user/":                 routeClass("DELETE", "/api/v1/user/", routes.AuthorizationFreshIdentity, ""),
		"DELETE /api/v1/user/:id":              routeClass("DELETE", "/api/v1/user/:id", routes.AuthorizationFreshAuthorization, valueobject.PermUserDelete),
		"PUT /api/v1/user/:id":                 routeClass("PUT", "/api/v1/user/:id", routes.AuthorizationFreshAuthorization, valueobject.PermFullAccess),
		"PATCH /api/v1/user/change-email":      routeClass("PATCH", "/api/v1/user/change-email", routes.AuthorizationFreshIdentity, ""),
		"PATCH /api/v1/user/change-password":   routeClass("PATCH", "/api/v1/user/change-password", routes.AuthorizationFreshIdentity, ""),
		"PATCH /api/v1/user/:id/change-role":   routeClass("PATCH", "/api/v1/user/:id/change-role", routes.AuthorizationFreshAuthorization, valueobject.PermUserChangeRole),
		"PATCH /api/v1/user/:id/change-status": routeClass("PATCH", "/api/v1/user/:id/change-status", routes.AuthorizationFreshAuthorization, valueobject.PermUserChangeStatus),
		"GET /api/v1/user/sessions":            routeClass("GET", "/api/v1/user/sessions", routes.AuthorizationFreshIdentity, ""),
		"DELETE /api/v1/user/sessions":         routeClass("DELETE", "/api/v1/user/sessions", routes.AuthorizationFreshIdentity, ""),
		"GET /api/v1/permission/":              routeClass("GET", "/api/v1/permission/", routes.AuthorizationFreshAuthorization, valueobject.PermFullAccess),
		"GET /api/v1/permission/:role":         routeClass("GET", "/api/v1/permission/:role", routes.AuthorizationFreshAuthorization, valueobject.PermFullAccess),
		"POST /api/v1/permission/":             routeClass("POST", "/api/v1/permission/", routes.AuthorizationFreshAuthorization, valueobject.PermFullAccess),
		"DELETE /api/v1/permission/:id":        routeClass("DELETE", "/api/v1/permission/:id", routes.AuthorizationFreshAuthorization, valueobject.PermFullAccess),
	}

	classified := make(map[string]routes.RouteClassification)
	for _, classification := range server.RouteClassifications() {
		key := classification.Method + " " + classification.Path
		if _, duplicate := classified[key]; duplicate {
			t.Fatalf("route %s has duplicate authorization classifications", key)
		}
		classified[key] = classification
	}
	if len(classified) != len(expected) {
		t.Fatalf("classified route count = %d, want %d; got %#v", len(classified), len(expected), classified)
	}
	for key, want := range expected {
		if got, ok := classified[key]; !ok {
			t.Errorf("registered route %s has no explicit authorization classification", key)
		} else if got != want {
			t.Errorf("classification for %s = %#v, want %#v", key, got, want)
		}
	}

	registered := server.engine.Routes()
	if len(registered) != len(classified) {
		t.Fatalf("Gin route count = %d, classified route count = %d", len(registered), len(classified))
	}
	for _, route := range registered {
		key := route.Method + " " + route.Path
		if _, ok := classified[key]; !ok {
			t.Errorf("Gin route %s bypassed the classification registry", key)
		}
	}
}

func TestNewServerAppliesConfiguredGinMode(t *testing.T) {
	previousMode := gin.Mode()
	t.Cleanup(func() {
		gin.SetMode(previousMode)
	})

	tests := []struct {
		name string
		mode GinMode
		want string
	}{
		{name: "debug", mode: GinModeDebug, want: gin.DebugMode},
		{name: "release", mode: GinModeRelease, want: gin.ReleaseMode},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			server, err := NewServer(
				testServerConfig(test.mode),
				ServerDependencies{
					MetricsService:   routeClassificationMetrics{registry: prometheus.NewRegistry()},
					ReadinessChecker: readinessCheckFunc(func(context.Context) error { return nil }),
				},
				middleware.RateLimitConfig{},
			)
			if err != nil {
				t.Fatalf("NewServer() error = %v", err)
			}
			if server == nil || server.engine == nil {
				t.Fatal("NewServer() returned no Gin engine")
			}
			if got := gin.Mode(); got != test.want {
				t.Fatalf("Gin mode = %q, want %q", got, test.want)
			}
		})
	}
}

func TestNewServerRejectsUnsupportedGinMode(t *testing.T) {
	previousMode := gin.Mode()
	config := testServerConfig(GinMode("unsupported"))
	server, err := NewServer(
		config,
		ServerDependencies{
			ReadinessChecker: readinessCheckFunc(func(context.Context) error { return nil }),
		},
		middleware.RateLimitConfig{},
	)
	if err == nil {
		t.Fatal("NewServer() error = nil, want unsupported Gin mode error")
	}
	if server != nil {
		t.Fatal("NewServer() returned a server for an unsupported Gin mode")
	}
	if got := gin.Mode(); got != previousMode {
		t.Fatalf("Gin mode changed to %q after invalid input; want %q", got, previousMode)
	}
}

func TestNewServerRequiresReadinessChecker(t *testing.T) {
	server, err := NewServer(
		testServerConfig(GinModeRelease),
		ServerDependencies{},
		middleware.RateLimitConfig{},
	)
	if !errors.Is(err, ErrReadinessCheckerRequired) {
		t.Fatalf("NewServer() error = %v, want ErrReadinessCheckerRequired", err)
	}
	if server != nil {
		t.Fatal("NewServer() returned a server without a readiness checker")
	}
}

func TestNewServerConstructsDeliveryComponentsFromNamedDependencies(t *testing.T) {
	metricsService := routeClassificationMetrics{registry: prometheus.NewRegistry()}
	rateLimitConfig := middleware.RateLimitConfig{
		Auth:    middleware.RateLimit{Limit: 11, Window: time.Minute},
		Public:  middleware.RateLimit{Limit: 22, Window: 2 * time.Minute},
		Private: middleware.RateLimit{Limit: 33, Window: 3 * time.Minute},
	}

	server, err := NewServer(
		testServerConfig(GinModeRelease),
		ServerDependencies{
			MetricsService:   metricsService,
			ReadinessChecker: readinessCheckFunc(func(context.Context) error { return nil }),
		},
		rateLimitConfig,
	)
	if err != nil {
		t.Fatalf("NewServer() error = %v", err)
	}
	if server.metricsService != metricsService {
		t.Fatal("NewServer() did not retain the named metrics dependency")
	}
	if server.rlConfig != rateLimitConfig {
		t.Fatalf("rate-limit config = %#v, want %#v", server.rlConfig, rateLimitConfig)
	}
	if server.authHandler == nil || server.userHandler == nil ||
		server.sessionHandler == nil || server.permissionHandler == nil {
		t.Fatal("NewServer() did not construct every handler")
	}
	if server.authMiddleware == nil || server.permMiddleware == nil ||
		server.metricsMiddleware == nil || server.rateLimitMiddleware == nil {
		t.Fatal("NewServer() did not construct every middleware")
	}
}

func TestNewServerAppliesHTTPIngressConfiguration(t *testing.T) {
	config := testServerConfig(GinModeRelease)
	config.ReadHeaderTimeout = 4 * time.Second
	config.ReadTimeout = 12 * time.Second
	config.WriteTimeout = 25 * time.Second
	config.IdleTimeout = 55 * time.Second
	config.MaxHeaderBytes = 32 << 10

	server, err := NewServer(
		config,
		ServerDependencies{
			MetricsService:   routeClassificationMetrics{registry: prometheus.NewRegistry()},
			ReadinessChecker: readinessCheckFunc(func(context.Context) error { return nil }),
		},
		middleware.RateLimitConfig{},
	)
	if err != nil {
		t.Fatalf("NewServer() error = %v", err)
	}

	if got := server.httpServer.ReadHeaderTimeout; got != config.ReadHeaderTimeout {
		t.Errorf("ReadHeaderTimeout = %s, want %s", got, config.ReadHeaderTimeout)
	}
	if got := server.httpServer.ReadTimeout; got != config.ReadTimeout {
		t.Errorf("ReadTimeout = %s, want %s", got, config.ReadTimeout)
	}
	if got := server.httpServer.WriteTimeout; got != config.WriteTimeout {
		t.Errorf("WriteTimeout = %s, want %s", got, config.WriteTimeout)
	}
	if got := server.httpServer.IdleTimeout; got != config.IdleTimeout {
		t.Errorf("IdleTimeout = %s, want %s", got, config.IdleTimeout)
	}
	if got := server.httpServer.MaxHeaderBytes; got != config.MaxHeaderBytes {
		t.Errorf("MaxHeaderBytes = %d, want %d", got, config.MaxHeaderBytes)
	}
}

func TestNewServerRejectsInvalidHTTPIngressConfiguration(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*ServerConfig)
	}{
		{name: "read-header timeout", mutate: func(config *ServerConfig) { config.ReadHeaderTimeout = 0 }},
		{name: "read timeout", mutate: func(config *ServerConfig) { config.ReadTimeout = 0 }},
		{name: "read-header exceeds read", mutate: func(config *ServerConfig) { config.ReadHeaderTimeout = config.ReadTimeout + time.Second }},
		{name: "write timeout", mutate: func(config *ServerConfig) { config.WriteTimeout = 0 }},
		{name: "idle timeout", mutate: func(config *ServerConfig) { config.IdleTimeout = 0 }},
		{name: "readiness timeout", mutate: func(config *ServerConfig) { config.ReadinessTimeout = 0 }},
		{name: "header bytes", mutate: func(config *ServerConfig) { config.MaxHeaderBytes = 0 }},
		{name: "body bytes", mutate: func(config *ServerConfig) { config.MaxRequestBodyBytes = 0 }},
		{name: "trusted proxy", mutate: func(config *ServerConfig) { config.TrustedProxies = []string{"not-an-ip"} }},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			config := testServerConfig(GinModeRelease)
			test.mutate(&config)
			server, err := NewServer(
				config,
				ServerDependencies{
					ReadinessChecker: readinessCheckFunc(func(context.Context) error { return nil }),
				},
				middleware.RateLimitConfig{},
			)
			if err == nil {
				t.Fatal("NewServer() error = nil, want invalid ingress configuration error")
			}
			if server != nil {
				t.Fatal("NewServer() returned a server for invalid ingress configuration")
			}
		})
	}
}

func TestNewServerLimitsRequestBodyReads(t *testing.T) {
	config := testServerConfig(GinModeRelease)
	config.MaxRequestBodyBytes = 8
	server, err := NewServer(
		config,
		ServerDependencies{
			MetricsService:   routeClassificationMetrics{registry: prometheus.NewRegistry()},
			ReadinessChecker: readinessCheckFunc(func(context.Context) error { return nil }),
		},
		middleware.RateLimitConfig{},
	)
	if err != nil {
		t.Fatalf("NewServer() error = %v", err)
	}

	var readErr error
	server.engine.POST("/body-limit-test", func(c *gin.Context) {
		_, readErr = io.ReadAll(c.Request.Body)
		c.Status(netHTTP.StatusNoContent)
	})

	exactRequest := httptest.NewRequest(netHTTP.MethodPost, "/body-limit-test", strings.NewReader("12345678"))
	server.engine.ServeHTTP(httptest.NewRecorder(), exactRequest)
	if readErr != nil {
		t.Fatalf("reading body at configured limit returned %v", readErr)
	}

	oversizedRequest := httptest.NewRequest(netHTTP.MethodPost, "/body-limit-test", strings.NewReader("123456789"))
	server.engine.ServeHTTP(httptest.NewRecorder(), oversizedRequest)
	var maxBytesErr *netHTTP.MaxBytesError
	if !errors.As(readErr, &maxBytesErr) {
		t.Fatalf("reading oversized body error = %v, want *http.MaxBytesError", readErr)
	}
	if maxBytesErr.Limit != config.MaxRequestBodyBytes {
		t.Fatalf("body limit = %d, want %d", maxBytesErr.Limit, config.MaxRequestBodyBytes)
	}
}

func TestNewServerUsesForwardedClientIPOnlyFromTrustedProxy(t *testing.T) {
	tests := []struct {
		name           string
		trustedProxies []string
		wantClientIP   string
	}{
		{name: "untrusted proxy header ignored", wantClientIP: "198.51.100.10"},
		{
			name:           "trusted proxy header accepted",
			trustedProxies: []string{"198.51.100.10"},
			wantClientIP:   "203.0.113.25",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			config := testServerConfig(GinModeRelease)
			config.TrustedProxies = test.trustedProxies
			server, err := NewServer(
				config,
				ServerDependencies{
					MetricsService:   routeClassificationMetrics{registry: prometheus.NewRegistry()},
					ReadinessChecker: readinessCheckFunc(func(context.Context) error { return nil }),
				},
				middleware.RateLimitConfig{},
			)
			if err != nil {
				t.Fatalf("NewServer() error = %v", err)
			}

			server.engine.GET("/client-ip-test", func(c *gin.Context) {
				c.String(netHTTP.StatusOK, c.ClientIP())
			})
			request := httptest.NewRequest(netHTTP.MethodGet, "/client-ip-test", nil)
			request.RemoteAddr = "198.51.100.10:4321"
			request.Header.Set("X-Forwarded-For", "203.0.113.25")
			recorder := httptest.NewRecorder()
			server.engine.ServeHTTP(recorder, request)

			if got := recorder.Body.String(); got != test.wantClientIP {
				t.Fatalf("ClientIP() = %q, want %q", got, test.wantClientIP)
			}
		})
	}
}

func TestLivenessPreservesContractWithoutDependencyProbes(t *testing.T) {
	var calls atomic.Int64
	dependencies := testServerDependencies(readinessCheckFunc(func(context.Context) error {
		calls.Add(1)
		return errors.New("private dependency failure")
	}))
	dependencies.RateLimiter = unexpectedRateLimiter{t: t}
	server, err := NewServer(
		testServerConfig(GinModeRelease),
		dependencies,
		middleware.RateLimitConfig{
			Public: middleware.RateLimit{Limit: 100, Window: time.Minute},
		},
	)
	if err != nil {
		t.Fatalf("NewServer() error = %v", err)
	}

	recorder := httptest.NewRecorder()
	server.engine.ServeHTTP(
		recorder,
		httptest.NewRequest(netHTTP.MethodGet, "/api/health", nil),
	)

	if recorder.Code != netHTTP.StatusOK {
		t.Fatalf("GET /api/health status = %d, want 200", recorder.Code)
	}
	if got := recorder.Body.String(); got != `{"status":"ok"}` {
		t.Fatalf("GET /api/health body = %q, want exact liveness contract", got)
	}
	if calls.Load() != 0 {
		t.Fatalf("liveness dependency checks = %d, want 0", calls.Load())
	}
}

func TestReadinessRequiresServingStateAndHealthyDependencies(t *testing.T) {
	var calls atomic.Int64
	var checkErr error
	dependencies := testServerDependencies(readinessCheckFunc(func(context.Context) error {
		calls.Add(1)
		return checkErr
	}))
	dependencies.RateLimiter = unexpectedRateLimiter{t: t}
	server, err := NewServer(
		testServerConfig(GinModeRelease),
		dependencies,
		middleware.RateLimitConfig{},
	)
	if err != nil {
		t.Fatalf("NewServer() error = %v", err)
	}

	assertReadinessResponse(t, server, netHTTP.StatusServiceUnavailable, `{"status":"not_ready"}`)
	if calls.Load() != 0 {
		t.Fatalf("dependency checks before serving = %d, want 0", calls.Load())
	}

	server.accepting.Store(true)
	assertReadinessResponse(t, server, netHTTP.StatusOK, `{"status":"ready"}`)
	if calls.Load() != 1 {
		t.Fatalf("dependency checks while ready = %d, want 1", calls.Load())
	}

	privateErr := errors.New("database host and credential details")
	checkErr = privateErr
	recorder := assertReadinessResponse(
		t,
		server,
		netHTTP.StatusServiceUnavailable,
		`{"status":"not_ready"}`,
	)
	if strings.Contains(recorder.Body.String(), privateErr.Error()) {
		t.Fatalf("readiness response exposed internal dependency error: %s", recorder.Body.String())
	}
}

func TestReadinessUsesBoundedRequestOwnedContext(t *testing.T) {
	config := testServerConfig(GinModeRelease)
	config.ReadinessTimeout = 20 * time.Millisecond
	server, err := NewServer(
		config,
		testServerDependencies(readinessCheckFunc(func(ctx context.Context) error {
			deadline, ok := ctx.Deadline()
			if !ok {
				t.Fatal("readiness check received an unbounded context")
			}
			if remaining := time.Until(deadline); remaining > config.ReadinessTimeout {
				t.Fatalf("readiness deadline = %s, exceeds configured %s", remaining, config.ReadinessTimeout)
			}
			<-ctx.Done()
			return ctx.Err()
		})),
		middleware.RateLimitConfig{},
	)
	if err != nil {
		t.Fatalf("NewServer() error = %v", err)
	}
	server.accepting.Store(true)

	assertReadinessResponse(t, server, netHTTP.StatusServiceUnavailable, `{"status":"not_ready"}`)
}

func TestReadinessPreservesEarlierRequestCancellation(t *testing.T) {
	server, err := NewServer(
		testServerConfig(GinModeRelease),
		testServerDependencies(readinessCheckFunc(func(ctx context.Context) error {
			if !errors.Is(ctx.Err(), context.Canceled) {
				t.Fatalf("readiness context error = %v, want request cancellation", ctx.Err())
			}
			return ctx.Err()
		})),
		middleware.RateLimitConfig{},
	)
	if err != nil {
		t.Fatalf("NewServer() error = %v", err)
	}
	server.accepting.Store(true)

	requestCtx, cancelRequest := context.WithCancel(context.Background())
	cancelRequest()
	request := httptest.NewRequest(netHTTP.MethodGet, "/api/ready", nil).WithContext(requestCtx)
	recorder := httptest.NewRecorder()
	server.engine.ServeHTTP(recorder, request)

	if recorder.Code != netHTTP.StatusServiceUnavailable {
		t.Fatalf("GET /api/ready status = %d, want 503", recorder.Code)
	}
}

func assertReadinessResponse(
	t *testing.T,
	server *Server,
	wantStatus int,
	wantBody string,
) *httptest.ResponseRecorder {
	t.Helper()
	recorder := httptest.NewRecorder()
	server.engine.ServeHTTP(
		recorder,
		httptest.NewRequest(netHTTP.MethodGet, "/api/ready", nil),
	)
	if recorder.Code != wantStatus {
		t.Fatalf("GET /api/ready status = %d, want %d", recorder.Code, wantStatus)
	}
	if got := recorder.Body.String(); got != wantBody {
		t.Fatalf("GET /api/ready body = %q, want %q", got, wantBody)
	}
	if got := recorder.Header().Get("Cache-Control"); got != "no-store" {
		t.Fatalf("GET /api/ready Cache-Control = %q, want no-store", got)
	}
	if got := recorder.Header().Get("Content-Type"); got != "application/json; charset=utf-8" {
		t.Fatalf("GET /api/ready Content-Type = %q, want JSON", got)
	}
	return recorder
}

func testServerConfig(mode GinMode) ServerConfig {
	return ServerConfig{
		GinMode:             mode,
		ReadHeaderTimeout:   5 * time.Second,
		ReadTimeout:         15 * time.Second,
		WriteTimeout:        30 * time.Second,
		IdleTimeout:         time.Minute,
		ReadinessTimeout:    2 * time.Second,
		MaxHeaderBytes:      64 << 10,
		MaxRequestBodyBytes: 1 << 20,
	}
}

func testServerDependencies(checker ReadinessChecker) ServerDependencies {
	return ServerDependencies{
		MetricsService:   routeClassificationMetrics{registry: prometheus.NewRegistry()},
		RateLimiter:      allowingRateLimiter{},
		ReadinessChecker: checker,
	}
}

type readinessCheckFunc func(context.Context) error

func (check readinessCheckFunc) CheckReadiness(ctx context.Context) error {
	return check(ctx)
}

type allowingRateLimiter struct{}

func (allowingRateLimiter) Allow(
	context.Context,
	string,
	string,
	string,
	int,
	time.Duration,
) (bool, time.Duration, int64, error) {
	return true, time.Minute, 1, nil
}

type unexpectedRateLimiter struct {
	t *testing.T
}

func (l unexpectedRateLimiter) Allow(
	context.Context,
	string,
	string,
	string,
	int,
	time.Duration,
) (bool, time.Duration, int64, error) {
	l.t.Fatal("probe endpoint invoked the Redis-backed rate limiter")
	return false, 0, 0, nil
}

func routeClass(
	method string,
	path string,
	class routes.AuthorizationClass,
	permission valueobject.Permission,
) routes.RouteClassification {
	return routes.RouteClassification{
		Method: method, Path: path, Class: class, Permission: permission,
	}
}

type routeClassificationMetrics struct {
	registry *prometheus.Registry
}

func (routeClassificationMetrics) RecordHTTPRequest(float64, string, string, string) {}
func (routeClassificationMetrics) RecordHTTPActiveRequests(bool)                     {}
func (routeClassificationMetrics) RecordDBQuery(float64, string, string)             {}
func (routeClassificationMetrics) RecordUserLogin(bool)                              {}
func (routeClassificationMetrics) RecordTokenRefresh(bool)                           {}
func (m routeClassificationMetrics) GetRegistry() *prometheus.Registry               { return m.registry }

type blockingListener struct {
	closed chan struct{}
	once   sync.Once
}

func newBlockingListener() *blockingListener {
	return &blockingListener{
		closed: make(chan struct{}),
	}
}

func (l *blockingListener) Accept() (net.Conn, error) {
	<-l.closed
	return nil, net.ErrClosed
}

func (l *blockingListener) Close() error {
	l.once.Do(func() {
		close(l.closed)
	})
	return nil
}

func (*blockingListener) Addr() net.Addr {
	return testAddr("test")
}

type testAddr string

func (a testAddr) Network() string { return string(a) }
func (a testAddr) String() string  { return string(a) }

var _ io.Closer = (*blockingListener)(nil)
