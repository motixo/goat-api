package handlers

import (
	"github.com/gin-gonic/gin"
	"github.com/mot0x0/goth-api/internal/delivery/http/helper"
	"github.com/mot0x0/goth-api/internal/delivery/http/response"
	"github.com/mot0x0/goth-api/internal/domain/service"
	"github.com/mot0x0/goth-api/internal/domain/usecase/auth"
)

type AuthHandler struct {
	usecase auth.UseCase
	logger  service.Logger
}

func NewAuthHandler(usecase auth.UseCase, logger service.Logger) *AuthHandler {
	return &AuthHandler{
		usecase: usecase,
		logger:  logger,
	}
}

func (h *AuthHandler) Login(c *gin.Context) {
	helper.LogRequest(h.logger, c)
	var input auth.LoginInput
	if err := c.ShouldBindJSON(&input); err != nil {
		h.logger.Warn("invalid request payload", "endpoint", c.FullPath(), "ip", c.ClientIP(), "device", c.GetHeader("User-Agent"))
		response.BadRequest(c, "Invalid request payload")
		return
	}

	input.IP = c.ClientIP()
	input.Device = c.GetHeader("User-Agent")

	output, err := h.usecase.Login(c.Request.Context(), input)
	if err != nil {
		response.DomainError(c, err)
		return
	}

	response.OK(c, output)
}

func (h *AuthHandler) Register(c *gin.Context) {
	helper.LogRequest(h.logger, c)
	var input auth.RegisterInput
	if err := c.ShouldBindJSON(&input); err != nil {
		h.logger.Warn("invalid request payload", "endpoint", c.FullPath(), "ip", c.ClientIP(), "device", c.GetHeader("User-Agent"))
		response.BadRequest(c, "Invalid request payload")
		return
	}

	output, err := h.usecase.Signup(c.Request.Context(), input)
	if err != nil {
		response.DomainError(c, err)
		return
	}

	response.Created(c, output)
}

func (h *AuthHandler) Refresh(c *gin.Context) {
	helper.LogRequest(h.logger, c)
	var input auth.RefreshInput
	if err := c.ShouldBindJSON(&input); err != nil {
		h.logger.Warn("invalid request payload", "endpoint", c.FullPath(), "ip", c.ClientIP(), "device", c.GetHeader("User-Agent"))
		response.BadRequest(c, "Invalid request payload")
		return
	}

	input.IP = c.ClientIP()
	input.Device = c.GetHeader("User-Agent")
	output, err := h.usecase.Refresh(c.Request.Context(), input)
	if err != nil {
		h.logger.Warn("invalid request payload", "endpoint", c.FullPath(), "ip", c.ClientIP(), "device", c.GetHeader("User-Agent"))
		response.Unauthorized(c, "Invalid request payload")
		return
	}

	response.OK(c, output)
}

func (h *AuthHandler) Logout(c *gin.Context) {
	helper.LogRequest(h.logger, c)
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

	if err := h.usecase.Logout(c.Request.Context(), sessionID, userID); err != nil {
		response.Unauthorized(c, err.Error())
		return
	}

	response.OK(c, "logout successful")

}
