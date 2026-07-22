package handlers

import (
	"github.com/gin-gonic/gin"
	"github.com/motixo/goat-api/internal/delivery/http/helper"
	"github.com/motixo/goat-api/internal/delivery/http/response"
	"github.com/motixo/goat-api/internal/pkg"
	"github.com/motixo/goat-api/internal/usecase/auth"
)

type AuthHandler struct {
	usecase auth.UseCase
	logger  pkg.Logger
}

func NewAuthHandler(usecase auth.UseCase, logger pkg.Logger) *AuthHandler {
	return &AuthHandler{
		usecase: usecase,
		logger:  logger,
	}
}

func (h *AuthHandler) Login(c *gin.Context) {
	helper.LogRequest(h.logger, c)
	var request loginRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		h.logger.Warn("invalid request payload", "endpoint", c.FullPath(), "ip", c.ClientIP(), "device", c.GetHeader("User-Agent"))
		response.BadRequest(c, "Invalid request payload")
		return
	}

	input := auth.LoginInput{
		Email:    request.Email,
		Password: request.Password,
		IP:       c.ClientIP(),
		Device:   c.GetHeader("User-Agent"),
	}

	output, err := h.usecase.Login(c.Request.Context(), input)
	if err != nil {
		response.DomainError(c, err)
		return
	}

	response.OK(c, newLoginResponse(output))
}

func (h *AuthHandler) Register(c *gin.Context) {
	helper.LogRequest(h.logger, c)
	var request registerRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		h.logger.Warn("invalid request payload", "endpoint", c.FullPath(), "ip", c.ClientIP(), "device", c.GetHeader("User-Agent"))
		response.BadRequest(c, "Invalid request payload")
		return
	}

	output, err := h.usecase.Signup(c.Request.Context(), auth.RegisterInput{
		Email:    request.Email,
		Password: request.Password,
	})
	if err != nil {
		response.DomainError(c, err)
		return
	}

	response.Created(c, newAuthUserResponse(output))
}

func (h *AuthHandler) Refresh(c *gin.Context) {
	helper.LogRequest(h.logger, c)
	var request refreshRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		h.logger.Warn("invalid request payload", "endpoint", c.FullPath(), "ip", c.ClientIP(), "device", c.GetHeader("User-Agent"))
		response.BadRequest(c, "Invalid request payload")
		return
	}

	input := auth.RefreshInput{
		RefreshToken: request.RefreshToken,
		IP:           c.ClientIP(),
		Device:       c.GetHeader("User-Agent"),
	}
	output, err := h.usecase.Refresh(c.Request.Context(), input)
	if err != nil {
		h.logger.Warn("invalid request payload", "endpoint", c.FullPath(), "ip", c.ClientIP(), "device", c.GetHeader("User-Agent"))
		response.DomainError(c, err)
		return
	}

	response.OK(c, newRefreshResponse(output))
}

func (h *AuthHandler) Logout(c *gin.Context) {
	helper.LogRequest(h.logger, c)
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

	if err := h.usecase.Logout(c.Request.Context(), sessionID, userID); err != nil {
		response.Unauthorized(c, err.Error())
		return
	}

	response.OK(c, "logout successful")

}
