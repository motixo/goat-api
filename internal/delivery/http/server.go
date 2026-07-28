package http

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/motixo/goat-api/internal/delivery/http/handlers"
	"github.com/motixo/goat-api/internal/delivery/http/middleware"
	"github.com/motixo/goat-api/internal/delivery/http/routes"
	"github.com/motixo/goat-api/internal/domain/service"
	"github.com/motixo/goat-api/internal/pkg"
	"github.com/motixo/goat-api/internal/usecase/authentication"
	"github.com/motixo/goat-api/internal/usecase/authorization"
	"github.com/motixo/goat-api/internal/usecase/permission"
	"github.com/motixo/goat-api/internal/usecase/session"
	"github.com/motixo/goat-api/internal/usecase/user"
)

var (
	ErrServerAlreadyStarted         = errors.New("HTTP server already started")
	ErrHTTPServerStoppedBeforeReady = errors.New("HTTP server stopped before accepting connections")
)

// GinMode is a delivery-owned Gin engine mode.
type GinMode string

const (
	// GinModeDebug enables Gin's debug mode.
	GinModeDebug GinMode = GinMode(gin.DebugMode)
	// GinModeRelease enables Gin's release mode.
	GinModeRelease GinMode = GinMode(gin.ReleaseMode)
)

func newGinEngine(mode GinMode, trustedProxies []string) (*gin.Engine, error) {
	switch mode {
	case GinModeDebug, GinModeRelease:
		gin.SetMode(string(mode))
	default:
		return nil, fmt.Errorf("unsupported Gin mode %q", mode)
	}

	router := gin.New()
	if err := router.SetTrustedProxies(trustedProxies); err != nil {
		return nil, fmt.Errorf("configure trusted HTTP proxies: %w", err)
	}
	return router, nil
}

// ServerConfig contains only delivery-owned HTTP ingress settings. Process
// environment parsing and policy validation remain in the composition/config
// boundary.
type ServerConfig struct {
	GinMode             GinMode
	ReadHeaderTimeout   time.Duration
	ReadTimeout         time.Duration
	WriteTimeout        time.Duration
	IdleTimeout         time.Duration
	MaxHeaderBytes      int
	MaxRequestBodyBytes int64
	TrustedProxies      []string
}

func (c ServerConfig) validate() error {
	timeouts := []struct {
		name  string
		value time.Duration
	}{
		{name: "read-header timeout", value: c.ReadHeaderTimeout},
		{name: "read timeout", value: c.ReadTimeout},
		{name: "write timeout", value: c.WriteTimeout},
		{name: "idle timeout", value: c.IdleTimeout},
	}
	for _, timeout := range timeouts {
		if timeout.value <= 0 {
			return fmt.Errorf("HTTP %s must be positive", timeout.name)
		}
	}
	if c.ReadHeaderTimeout > c.ReadTimeout {
		return fmt.Errorf("HTTP read-header timeout must not exceed read timeout")
	}
	if c.MaxHeaderBytes <= 0 {
		return fmt.Errorf("HTTP maximum header bytes must be positive")
	}
	if c.MaxRequestBodyBytes <= 0 {
		return fmt.Errorf("HTTP maximum request body bytes must be positive")
	}
	return nil
}

type Server struct {
	engine               *gin.Engine
	httpServer           *http.Server
	authHandler          *handlers.AuthHandler
	userHandler          *handlers.UserHandler
	sessionHandler       *handlers.SessionHandler
	permissionHandler    *handlers.PermissionHandler
	authMiddleware       *middleware.AuthMiddleware
	permMiddleware       *middleware.PermMiddleware
	metricsMiddleware    *middleware.MetricsMiddleware
	rateLimitMiddleware  *middleware.RateLimitMiddleware
	rlConfig             middleware.RateLimitConfig
	metricsService       service.MetricsService
	routeClassifications []routes.RouteClassification

	lifecycleMu sync.Mutex
	started     bool
	listen      func(network, address string) (net.Listener, error)
}

// ServerDependencies names the application ports and shared services required
// to construct the HTTP delivery boundary. Runtime configuration remains
// explicit in NewServer rather than being mixed with long-lived collaborators.
type ServerDependencies struct {
	UserUseCase           user.UseCase
	AuthenticationUseCase authentication.UseCase
	AuthorizationUseCase  authorization.UseCase
	PermissionUseCase     permission.UseCase
	SessionUseCase        session.UseCase
	Logger                pkg.Logger
	JWTService            service.JWTService
	MetricsService        service.MetricsService
	RateLimiter           service.RateLimiter
}

type readyListener struct {
	net.Listener
	ready chan struct{}
	once  sync.Once
}

func (l *readyListener) Accept() (net.Conn, error) {
	l.once.Do(func() {
		close(l.ready)
	})
	return l.Listener.Accept()
}

func NewServer(
	config ServerConfig,
	dependencies ServerDependencies,
	rlConfig middleware.RateLimitConfig,
) (*Server, error) {
	if err := config.validate(); err != nil {
		return nil, err
	}
	trustedProxies := append([]string(nil), config.TrustedProxies...)
	router, err := newGinEngine(config.GinMode, trustedProxies)
	if err != nil {
		return nil, err
	}

	// Global middleware
	authMiddleware := middleware.NewAuthMiddleware(dependencies.JWTService, dependencies.SessionUseCase)
	permMiddleware := middleware.NewPermMiddleware(dependencies.AuthorizationUseCase)
	metricsMiddleware := middleware.NewMetricsMiddleware(dependencies.MetricsService)
	rateLimitMiddleware := middleware.NewRateLimitMiddleware(dependencies.RateLimiter, dependencies.Logger)

	router.Use(
		middleware.Recovery(dependencies.Logger),
		middleware.RequestBodyLimit(config.MaxRequestBodyBytes),
		metricsMiddleware.Handler(),
	)

	authHandler := handlers.NewAuthHandler(dependencies.AuthenticationUseCase, dependencies.Logger)
	sessionHandler := handlers.NewSessionHandler(dependencies.SessionUseCase, dependencies.Logger)
	userHandler := handlers.NewUserHandler(dependencies.UserUseCase, dependencies.Logger)
	permissionHandler := handlers.NewPermissionHandler(dependencies.PermissionUseCase, dependencies.Logger)

	httpServerInstance := &http.Server{
		Handler:           router,
		ReadHeaderTimeout: config.ReadHeaderTimeout,
		ReadTimeout:       config.ReadTimeout,
		WriteTimeout:      config.WriteTimeout,
		IdleTimeout:       config.IdleTimeout,
		MaxHeaderBytes:    config.MaxHeaderBytes,
	}

	server := &Server{
		engine:              router,
		httpServer:          httpServerInstance,
		authHandler:         authHandler,
		userHandler:         userHandler,
		sessionHandler:      sessionHandler,
		permissionHandler:   permissionHandler,
		authMiddleware:      authMiddleware,
		permMiddleware:      permMiddleware,
		metricsMiddleware:   metricsMiddleware,
		rateLimitMiddleware: rateLimitMiddleware,
		metricsService:      dependencies.MetricsService,
		rlConfig:            rlConfig,
		listen:              net.Listen,
	}

	server.setupRoutes()
	return server, nil
}

func (s *Server) setupRoutes() {
	api := s.engine.Group("/api")
	v1 := api.Group("/v1")
	classifications := routes.NewClassificationRegistry()
	routes.RegisterMetricsRoutes(api, s.metricsService, classifications)
	routes.RegisterUserRoutes(v1, s.userHandler, s.sessionHandler, s.authMiddleware, s.permMiddleware, s.rateLimitMiddleware, s.rlConfig, classifications)
	routes.RegisterAuthRoutes(v1, s.authHandler, s.authMiddleware, s.permMiddleware, s.rateLimitMiddleware, s.rlConfig, classifications)
	routes.RegisterPermissionRoutes(v1, s.permissionHandler, s.authMiddleware, s.permMiddleware, s.rateLimitMiddleware, s.rlConfig, classifications)

	// Health check
	classifications.Public(
		api,
		http.MethodGet,
		"/health",
		s.rateLimitMiddleware.Handler(s.rlConfig.Public),
		func(c *gin.Context) { c.JSON(200, gin.H{"status": "ok"}) },
	)
	s.routeClassifications = classifications.Entries()
}

func (s *Server) RouteClassifications() []routes.RouteClassification {
	return append([]routes.RouteClassification(nil), s.routeClassifications...)
}

// Start binds the listener synchronously so startup failures are returned
// before the application enters its running state.
func (s *Server) Start(addr string) (<-chan error, error) {
	s.lifecycleMu.Lock()
	defer s.lifecycleMu.Unlock()
	if s.started {
		return nil, ErrServerAlreadyStarted
	}

	s.httpServer.Addr = addr
	listen := s.listen
	if listen == nil {
		listen = net.Listen
	}
	listener, err := listen("tcp", addr)
	if err != nil {
		return nil, fmt.Errorf("listen on %s: %w", addr, err)
	}
	ready := make(chan struct{})
	listenerWithReady := &readyListener{
		Listener: listener,
		ready:    ready,
	}

	serveErrors := make(chan error, 1)
	go func() {
		err := s.httpServer.Serve(listenerWithReady)
		if errors.Is(err, http.ErrServerClosed) {
			err = nil
		}
		serveErrors <- err
		close(serveErrors)
	}()

	select {
	case <-ready:
		s.started = true
		return serveErrors, nil
	case serveErr := <-serveErrors:
		if serveErr == nil {
			serveErr = ErrHTTPServerStoppedBeforeReady
		}
		return nil, fmt.Errorf("start HTTP server: %w", serveErr)
	}
}

func (s *Server) Shutdown(ctx context.Context) error {
	err := s.httpServer.Shutdown(ctx)
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return err
}

func (s *Server) Close() error {
	err := s.httpServer.Close()
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return err
}
