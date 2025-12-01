package handlers

import (
	"github.com/gin-gonic/gin"
	"github.com/mot0x0/goth-api/internal/delivery/http/helper"
	"github.com/mot0x0/goth-api/internal/delivery/http/response"
	"github.com/mot0x0/goth-api/internal/domain/service"
	"github.com/mot0x0/goth-api/internal/domain/usecase/session"
)

type SessionHandler struct {
	usecase session.UseCase
	logger  service.Logger
}

func NewSessionHandler(usecase session.UseCase, logger service.Logger) *SessionHandler {
	return &SessionHandler{
		usecase: usecase,
		logger:  logger,
	}
}

func (s *SessionHandler) GetAllUserSessions(c *gin.Context) {
	helper.LogRequest(s.logger, c)
	userID := c.GetString("user_id")
	if userID == "" {
		response.Internal(c)
		return
	}

	sessionID := c.GetString("session_id")
	if sessionID == "" {
		response.Internal(c)
		return
	}

	output, err := s.usecase.GetSessionsByUser(c, userID, sessionID)
	if err != nil {
		response.Internal(c)
		return
	}
	response.OK(c, output)

}

func (s *SessionHandler) DeleteSessions(c *gin.Context) {
	helper.LogRequest(s.logger, c)
	var input session.DeleteSessionsInput
	if err := c.ShouldBindJSON(&input); err != nil {
		s.logger.Warn("invalid request payload", "endpoint", c.FullPath(), "ip", c.ClientIP(), "device", c.GetHeader("User-Agent"))
		response.BadRequest(c, "Invalid request payload")
		return
	}

	userID := c.GetString("user_id")
	if userID == "" {
		response.Internal(c)
		return
	}
	sessionID := c.GetString("session_id")
	if sessionID == "" {
		response.Internal(c)
		return
	}
	input.UserID = userID
	input.CurrentSession = sessionID

	if err := s.usecase.DeleteSessions(c, input); err != nil {
		response.Internal(c)
		return
	}
	response.OK(c, "Revoked")
}
