package handlers

import (
	"github.com/gin-gonic/gin"
	"github.com/motixo/goat-api/internal/delivery/http/helper"
	"github.com/motixo/goat-api/internal/delivery/http/response"
	"github.com/motixo/goat-api/internal/domain/usecase/session"
	"github.com/motixo/goat-api/internal/infra/logger"
)

type SessionHandler struct {
	usecase session.UseCase
	logger  logger.Logger
}

func NewSessionHandler(usecase session.UseCase, logger logger.Logger) *SessionHandler {
	return &SessionHandler{
		usecase: usecase,
		logger:  logger,
	}
}

func (h *SessionHandler) GetAllUserSessions(c *gin.Context) {
	helper.LogRequest(h.logger, c)
	var input helper.PaginationInput
	if err := c.ShouldBindQuery(&input); err != nil {
		response.BadRequest(c, "invalid pagination params")
		return
	}
	input.Validate()
	userID := c.GetString("user_id")
	if userID == "" {
		response.Unauthorized(c, "authentication context missing")
		return
	}

	sessionID := c.GetString("session_id")
	if sessionID == "" {
		response.Unauthorized(c, "authentication context missing")
		return
	}

	output, total, err := h.usecase.GetSessionsByUser(c, userID, sessionID, input.Offset(), input.Limit)
	if err != nil {
		response.Internal(c)
		return
	}
	meta := helper.NewPaginationMeta(total, input)
	response.OK(c, gin.H{"data": output, "meta": meta})
}

func (h *SessionHandler) DeleteSessions(c *gin.Context) {
	helper.LogRequest(h.logger, c)
	var input session.DeleteSessionsInput
	if err := c.ShouldBindJSON(&input); err != nil {
		h.logger.Warn("invalid request payload", "endpoint", c.FullPath(), "ip", c.ClientIP(), "device", c.GetHeader("User-Agent"))
		response.BadRequest(c, "Invalid request payload")
		return
	}
	if !input.RemoveOthers && len(input.TargetSessions) == 0 {
		h.logger.Warn("invalid request payload", "endpoint", c.FullPath(), "ip", c.ClientIP(), "device", c.GetHeader("User-Agent"))
		response.BadRequest(c, "Invalid request payload")
		return
	}

	userID := c.GetString("user_id")
	if userID == "" {
		response.Unauthorized(c, "authentication context missing")
		return
	}
	sessionID := c.GetString("session_id")
	if sessionID == "" {
		response.Unauthorized(c, "authentication context missing")
		return
	}
	input.UserID = userID
	input.CurrentSession = sessionID

	if err := h.usecase.DeleteSessions(c, input); err != nil {
		response.Internal(c)
		return
	}
	response.OK(c, "Revoked")
}
