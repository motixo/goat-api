package http

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/motixo/goat-api/internal/delivery/http/handlers"
	"github.com/motixo/goat-api/internal/delivery/http/middleware"
	"github.com/motixo/goat-api/internal/delivery/http/routes"
	"github.com/motixo/goat-api/internal/domain/service"
	"github.com/motixo/goat-api/internal/domain/usecase/auth"
	"github.com/motixo/goat-api/internal/domain/usecase/permission"
	"github.com/motixo/goat-api/internal/domain/usecase/session"
	"github.com/motixo/goat-api/internal/domain/usecase/user"
)

type Server struct {
	engine            *gin.Engine
	httpServer        *http.Server
	authHandler       *handlers.AuthHandler
	userHandler       *handlers.UserHandler
	sessionHandler    *handlers.SessionHandler
	permissionHandler *handlers.PermissionHandler
	authMiddleware    *middleware.AuthMiddleware
	permMiddleware    *middleware.PermMiddleware
	metricsMiddleware *middleware.MetricsMiddleware
	metricsService    service.MetricsService
}

func NewServer(
	userUC user.UseCase,
	authUC auth.UseCase,
	permUC permission.UseCase,
	permCache service.PermCacheService,
	sessionUC session.UseCase,
	userCache service.UserCacheService,
	logger service.Logger,
	jwtService service.JWTService,
	metricsService service.MetricsService,
) *Server {
	router := gin.New()

	// Global middleware
	authMiddleware := middleware.NewAuthMiddleware(jwtService, sessionUC, userCache)
	permMiddleware := middleware.NewPermMiddleware(userUC, permCache, userCache)
	metricsMiddleware := middleware.NewMetricsMiddleware(metricsService)
	router.Use(
		middleware.Recovery(logger),
		metricsMiddleware.Handler(),
	)

	authHandler := handlers.NewAuthHandler(authUC, logger)
	sessionHandler := handlers.NewSessionHandler(sessionUC, logger)
	userHandler := handlers.NewUserHandler(userUC, logger)
	permissionHandler := handlers.NewPermissionHandler(permUC, logger)

	httpServerInstance := &http.Server{
		Handler:      router,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	server := &Server{
		engine:            router,
		httpServer:        httpServerInstance,
		authHandler:       authHandler,
		userHandler:       userHandler,
		sessionHandler:    sessionHandler,
		permissionHandler: permissionHandler,
		authMiddleware:    authMiddleware,
		permMiddleware:    permMiddleware,
		metricsMiddleware: metricsMiddleware,
		metricsService:    metricsService,
	}

	server.setupRoutes()
	return server
}

func (s *Server) setupRoutes() {
	api := s.engine.Group("/api")
	v1 := api.Group("/v1")
	routes.RegisterMetricsRoutes(api, s.metricsService)
	routes.RegisterUserRoutes(v1, s.userHandler, s.sessionHandler, s.authMiddleware, s.permMiddleware)
	routes.RegisterAuthRoutes(v1, s.authHandler, s.authMiddleware, s.permMiddleware)
	routes.RegisterPermissionRoutes(v1, s.permissionHandler, s.authMiddleware, s.permMiddleware)

	// Health check
	s.engine.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok"})
	})
}

func (s *Server) Run(addr string) error {
	s.httpServer.Addr = addr

	if err := s.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return err
	}
	return nil
}

func (s *Server) Shutdown(ctx context.Context) error {
	return s.httpServer.Shutdown(ctx)
}
