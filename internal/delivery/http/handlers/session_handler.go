package handlers

import (
	"github.com/gin-gonic/gin"
	"github.com/mot0x0/gopi/internal/delivery/http/response"
	"github.com/mot0x0/gopi/internal/domain/usecase/session"
)

type SessionHandler struct {
	usecase   session.UseCase
	jwtSecret string
}

func NewSessionHandler(usecase session.UseCase, jwtSecret string) *SessionHandler {
	return &SessionHandler{
		usecase:   usecase,
		jwtSecret: jwtSecret,
	}
}

func (s *SessionHandler) GetAllUserSessions(c *gin.Context) {
	userID, _ := c.Get("user_id")

	output, err := s.usecase.GetSessionsByUser(c, userID.(string))
	if err != nil {
		response.Internal(c)
		return
	}
	response.OK(c, output)

}
