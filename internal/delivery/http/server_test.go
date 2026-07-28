package http

import (
	"context"
	"errors"
	"io"
	"net"
	netHTTP "net/http"
	"sync"
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

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := server.Shutdown(ctx); err != nil {
		t.Fatalf("Shutdown() error = %v", err)
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
		GinModeRelease,
		ServerDependencies{
			MetricsService: routeClassificationMetrics{registry: prometheus.NewRegistry()},
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
				test.mode,
				ServerDependencies{
					MetricsService: routeClassificationMetrics{registry: prometheus.NewRegistry()},
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
	server, err := NewServer(
		GinMode("unsupported"),
		ServerDependencies{},
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

func TestNewServerConstructsDeliveryComponentsFromNamedDependencies(t *testing.T) {
	metricsService := routeClassificationMetrics{registry: prometheus.NewRegistry()}
	rateLimitConfig := middleware.RateLimitConfig{
		Auth:    middleware.RateLimit{Limit: 11, Window: time.Minute},
		Public:  middleware.RateLimit{Limit: 22, Window: 2 * time.Minute},
		Private: middleware.RateLimit{Limit: 33, Window: 3 * time.Minute},
	}

	server, err := NewServer(
		GinModeRelease,
		ServerDependencies{MetricsService: metricsService},
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
