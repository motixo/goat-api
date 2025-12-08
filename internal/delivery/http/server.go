package http

import (
	"github.com/gin-gonic/gin"
	"github.com/motixo/goat-api/internal/delivery/http/handlers"
	"github.com/motixo/goat-api/internal/delivery/http/middleware"
	"github.com/motixo/goat-api/internal/delivery/http/routes"
	"github.com/motixo/goat-api/internal/domain/repository"
	"github.com/motixo/goat-api/internal/domain/service"
	"github.com/motixo/goat-api/internal/domain/usecase/auth"
	"github.com/motixo/goat-api/internal/domain/usecase/permission"
	"github.com/motixo/goat-api/internal/domain/usecase/session"
	"github.com/motixo/goat-api/internal/domain/usecase/user"
	"github.com/motixo/goat-api/internal/infrastructure/logger"
)

type Server struct {
	engine         *gin.Engine
	authHandler    *handlers.AuthHandler
	userHandler    *handlers.UserHandler
	sessionHandler *handlers.SessionHandler
	authMiddleware *middleware.AuthMiddleware
	permMiddleware *middleware.PermMiddleware
}

func NewServer(
	userUC user.UseCase,
	authUC auth.UseCase,
	permUC permission.UseCase,
	sessionUC session.UseCase,
	roleCache repository.RoleRepository,
	logger logger.Logger,
	jwtService service.JWTService,
) *Server {
	router := gin.New()

	// Global middleware
	authMiddleware := middleware.NewAuthMiddleware(jwtService, sessionUC)
	permMiddleware := middleware.NewPermMiddleware(userUC, permUC, roleCache)
	router.Use(middleware.Recovery(logger))

	authHandler := handlers.NewAuthHandler(authUC, logger)
	sessionHandler := handlers.NewSessionHandler(sessionUC, logger)
	userHandler := handlers.NewUserHandler(userUC, logger)

	server := &Server{
		engine:         router,
		authHandler:    authHandler,
		userHandler:    userHandler,
		authMiddleware: authMiddleware,
		permMiddleware: permMiddleware,
		sessionHandler: sessionHandler,
	}

	server.setupRoutes()
	return server
}

func (s *Server) setupRoutes() {
	api := s.engine.Group("/api")
	v1 := api.Group("/v1")

	routes.RegisterUserRoutes(v1, s.userHandler, s.sessionHandler, s.authMiddleware, s.permMiddleware)
	routes.RegisterAuthRoutes(v1, s.authHandler, s.authMiddleware, s.permMiddleware)

	// Health check
	s.engine.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok"})
	})
}

func (s *Server) Run(addr string) error {
	return s.engine.Run(addr)
}
