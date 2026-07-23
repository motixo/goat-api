package handlers

import (
	"github.com/gin-gonic/gin"
	"github.com/motixo/goat-api/internal/delivery/http/helper"
	"github.com/motixo/goat-api/internal/delivery/http/response"
	"github.com/motixo/goat-api/internal/pkg"
	"github.com/motixo/goat-api/internal/usecase/session"
)

type SessionHandler struct {
	usecase session.UseCase
	logger  pkg.Logger
}

func NewSessionHandler(usecase session.UseCase, logger pkg.Logger) *SessionHandler {
	return &SessionHandler{
		usecase: usecase,
		logger:  logger,
	}
}

func (h *SessionHandler) GetAllUserSessions(c *gin.Context) {
	helper.LogRequest(h.logger, c)
	var input helper.PaginationInput
	if err := c.ShouldBindQuery(&input); err != nil {
		response.BadRequest(c, response.DetailInvalidPaginationParams)
		return
	}
	input.Validate()
	userID := c.GetString("user_id")
	if userID == "" {
		response.Unauthorized(c, response.DetailAuthenticationContextMissing)
		return
	}

	sessionID := c.GetString("session_id")
	if sessionID == "" {
		response.Unauthorized(c, response.DetailAuthenticationContextMissing)
		return
	}

	output, total, err := h.usecase.GetSessionsByUser(c, userID, sessionID, input.Offset(), input.Limit)
	if err != nil {
		response.WriteProblem(c, response.MapError(err))
		return
	}
	meta := helper.NewPaginationMeta(total, input)
	response.OK(c, gin.H{"data": newSessionResponses(output), "meta": meta})
}

func (h *SessionHandler) DeleteSessions(c *gin.Context) {
	helper.LogRequest(h.logger, c)
	var request deleteSessionsRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		h.logger.Warn("invalid request payload", "endpoint", c.FullPath(), "ip", c.ClientIP(), "device", c.GetHeader("User-Agent"))
		response.BadRequest(c, response.DetailInvalidRequestPayload)
		return
	}
	if !request.Others && len(request.SessionIDs) == 0 {
		h.logger.Warn("invalid request payload", "endpoint", c.FullPath(), "ip", c.ClientIP(), "device", c.GetHeader("User-Agent"))
		response.BadRequest(c, response.DetailInvalidRequestPayload)
		return
	}

	userID := c.GetString("user_id")
	if userID == "" {
		response.Unauthorized(c, response.DetailAuthenticationContextMissing)
		return
	}
	sessionID := c.GetString("session_id")
	if sessionID == "" {
		response.Unauthorized(c, response.DetailAuthenticationContextMissing)
		return
	}
	input := session.DeleteSessionsInput{
		UserID:         userID,
		CurrentSession: sessionID,
		TargetSessions: request.SessionIDs,
		RemoveOthers:   request.Others,
	}

	if err := h.usecase.DeleteSessions(c, input); err != nil {
		response.WriteProblem(c, response.MapError(err))
		return
	}
	response.OK(c, "Revoked")
}
