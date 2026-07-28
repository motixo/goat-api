package handlers

import (
	"github.com/gin-gonic/gin"
	"github.com/motixo/goat-api/internal/delivery/http/helper"
	"github.com/motixo/goat-api/internal/delivery/http/middleware"
	"github.com/motixo/goat-api/internal/delivery/http/response"
	"github.com/motixo/goat-api/internal/pkg"
	"github.com/motixo/goat-api/internal/usecase/authentication"
)

type AuthHandler struct {
	usecase authentication.UseCase
	logger  pkg.Logger
}

func NewAuthHandler(usecase authentication.UseCase, logger pkg.Logger) *AuthHandler {
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
		response.BadRequest(c, response.DetailInvalidRequestPayload)
		return
	}

	input := authentication.LoginInput{
		Email:    request.Email,
		Password: request.Password,
		IP:       c.ClientIP(),
		Device:   c.GetHeader("User-Agent"),
	}

	output, err := h.usecase.Login(c.Request.Context(), input)
	if err != nil {
		response.WriteProblem(c, response.MapError(err))
		return
	}

	response.OK(c, newLoginResponse(output))
}

func (h *AuthHandler) Register(c *gin.Context) {
	helper.LogRequest(h.logger, c)
	var request registerRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		h.logger.Warn("invalid request payload", "endpoint", c.FullPath(), "ip", c.ClientIP(), "device", c.GetHeader("User-Agent"))
		response.BadRequest(c, response.DetailInvalidRequestPayload)
		return
	}

	output, err := h.usecase.Signup(c.Request.Context(), authentication.RegisterInput{
		Email:    request.Email,
		Password: request.Password,
	})
	if err != nil {
		response.WriteProblem(c, response.MapError(err))
		return
	}

	response.Created(c, newAuthUserResponse(output))
}

func (h *AuthHandler) Refresh(c *gin.Context) {
	helper.LogRequest(h.logger, c)
	var request refreshRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		h.logger.Warn("invalid request payload", "endpoint", c.FullPath(), "ip", c.ClientIP(), "device", c.GetHeader("User-Agent"))
		response.BadRequest(c, response.DetailInvalidRequestPayload)
		return
	}

	input := authentication.RefreshInput{
		RefreshToken: request.RefreshToken,
		IP:           c.ClientIP(),
		Device:       c.GetHeader("User-Agent"),
	}
	output, err := h.usecase.Refresh(c.Request.Context(), input)
	if err != nil {
		h.logger.Warn("invalid request payload", "endpoint", c.FullPath(), "ip", c.ClientIP(), "device", c.GetHeader("User-Agent"))
		response.WriteProblem(c, response.MapError(err))
		return
	}

	response.OK(c, newRefreshResponse(output))
}

func (h *AuthHandler) Logout(c *gin.Context) {
	helper.LogRequest(h.logger, c)
	principal, ok := middleware.PrincipalFrom(c)
	if !ok {
		response.Unauthorized(c, response.DetailAuthenticationContextMissing)
		return
	}

	if err := h.usecase.Logout(
		c.Request.Context(),
		principal.SessionID(),
		principal.UserID(),
	); err != nil {
		response.WriteProblem(c, response.MapError(err))
		return
	}

	response.OK(c, "logout successful")

}
