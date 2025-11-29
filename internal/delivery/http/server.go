package http

import (
	"github.com/gin-gonic/gin"
	"github.com/mot0x0/goth-api/internal/config"
	"github.com/mot0x0/goth-api/internal/delivery/http/handlers"
	"github.com/mot0x0/goth-api/internal/delivery/http/middleware"
	"github.com/mot0x0/goth-api/internal/delivery/http/routes"
	"github.com/mot0x0/goth-api/internal/domain/usecase/auth"
	"github.com/mot0x0/goth-api/internal/domain/usecase/session"
	"github.com/mot0x0/goth-api/internal/domain/usecase/user"
)

type Server struct {
	engine         *gin.Engine
	authHandler    *handlers.AuthHandler
	userHandler    *handlers.UserHandler
	sessionHandler *handlers.SessionHandler
	authMiddleware *middleware.AuthMiddleware
}

func NewServer(userUC user.UseCase, authUC auth.UseCase, sessionUC session.UseCase, cfg *config.Config) *Server {
	router := gin.Default()

	// Global middleware
	authMiddleware := middleware.NewAuthMiddleware(cfg.JWTSecret, sessionUC)
	router.Use(middleware.Recovery())

	authHandler := handlers.NewAuthHandler(authUC)
	sessionHandler := handlers.NewSessionHandler(sessionUC)
	userHandler := handlers.NewUserHandler(userUC)

	server := &Server{
		engine:         router,
		authHandler:    authHandler,
		userHandler:    userHandler,
		authMiddleware: authMiddleware,
		sessionHandler: sessionHandler,
	}

	server.setupRoutes()
	return server
}

func (s *Server) setupRoutes() {
	api := s.engine.Group("/api")
	v1 := api.Group("/v1")

	routes.RegisterUserRoutes(v1, s.userHandler, s.sessionHandler, s.authMiddleware)
	routes.RegisterAuthRoutes(v1, s.authHandler, s.authMiddleware)

	// Health check
	s.engine.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok"})
	})
}

func (s *Server) Run(addr string) error {
	return s.engine.Run(addr)
}
