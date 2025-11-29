package handlers

import (
	"github.com/gin-gonic/gin"
	"github.com/mot0x0/goth-api/internal/delivery/http/helper"
	"github.com/mot0x0/goth-api/internal/delivery/http/response"
	"github.com/mot0x0/goth-api/internal/domain/usecase/session"
)

type SessionHandler struct {
	usecase session.UseCase
}

func NewSessionHandler(usecase session.UseCase) *SessionHandler {
	return &SessionHandler{usecase: usecase}
}

func (s *SessionHandler) GetAllUserSessions(c *gin.Context) {

	userID, err := helper.GetStringFromContext(c, "user_id")
	if err != nil {
		response.Internal(c)
		return
	}

	sessionID, err := helper.GetStringFromContext(c, "session_id")
	if err != nil {
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
	var input session.DeleteSessionsInput
	if err := c.ShouldBindJSON(&input); err != nil {
		response.BadRequest(c, "Invalid request payload")
		return
	}

	userID, err := helper.GetStringFromContext(c, "user_id")
	if err != nil {
		response.Internal(c)
		return
	}
	sessionID, err := helper.GetStringFromContext(c, "session_id")
	if err != nil {
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
