package gin

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/mtextr/gopi/internal/domain"
	"github.com/mtextr/gopi/internal/usecase/user"
)

type Server struct {
	engine  *gin.Engine
	usersUC *user.UserUsecase
}

func NewServer(usersUC *user.UserUsecase) *Server {
	r := gin.Default()

	s := &Server{
		engine:  r,
		usersUC: usersUC,
	}

	s.registerRoutes()
	return s
}

func (s *Server) registerRoutes() {
	v1 := s.engine.Group("/api/v1")
	{
		v1.POST("/users", s.createUser)
		//v1.GET("/users/:id", s.getUserByID)
	}
}

func (s *Server) Run(addr string) error {
	return s.engine.Run(addr)
}

// ---------------------- Handlers ----------------------

func (s *Server) createUser(c *gin.Context) {
	var input domain.User
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	err := s.usersUC.Create(c.Request.Context(), &input)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, input)
}

// func (s *Server) getUserByID(c *gin.Context) {
// 	id := c.Param("id")

// 	user := domain.User{
// 		ID:    id,
// 		Email: "ali@example.com",
// 	}

// 	c.JSON(http.StatusOK, user)
// }
