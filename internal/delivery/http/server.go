package http

import (
	"github.com/gin-gonic/gin"
	"github.com/mot0x0/gopi/internal/delivery/http/handlers"
	"github.com/mot0x0/gopi/internal/delivery/http/middleware"
	"github.com/mot0x0/gopi/internal/delivery/http/routes"
	"github.com/mot0x0/gopi/internal/domain/usecase/user"
)

type Server struct {
	engine      *gin.Engine
	userHandler *handlers.UserHandler
}

func NewServer(userUC user.UserUseCase) *Server {
	router := gin.Default()

	// Global middleware
	router.Use(middleware.Recovery())
	//router.Use(middleware.Logger())
	//router.Use(middleware.CORS())

	userHandler := handlers.NewUserHandler(userUC)

	server := &Server{
		engine:      router,
		userHandler: userHandler,
	}

	server.setupRoutes()
	return server
}

func (s *Server) setupRoutes() {
	api := s.engine.Group("/api")
	v1 := api.Group("/v1")

	routes.RegisterUserRoutes(v1, s.userHandler)

	// Health check
	s.engine.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok"})
	})
}

func (s *Server) Run(addr string) error {
	return s.engine.Run(addr)
}
