package handlers

import (
	"github.com/gin-gonic/gin"
	"github.com/mot0x0/goth-api/internal/delivery/http/response"
	"github.com/mot0x0/goth-api/internal/domain/usecase/auth"
)

type AuthHandler struct {
	usecase auth.UseCase
}

func NewAuthHandler(usecase auth.UseCase) *AuthHandler {
	return &AuthHandler{usecase: usecase}
}

func (h *AuthHandler) Login(c *gin.Context) {
	var input auth.LoginInput
	if err := c.ShouldBindJSON(&input); err != nil {
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
	var input auth.RegisterInput
	if err := c.ShouldBindJSON(&input); err != nil {
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
	var input auth.RefreshInput
	if err := c.ShouldBindJSON(&input); err != nil {
		response.BadRequest(c, "token is required")
		return
	}

	input.IP = c.ClientIP()
	input.Device = c.GetHeader("User-Agent")

	output, err := h.usecase.Refresh(c.Request.Context(), input)
	if err != nil {
		response.Unauthorized(c, "invalid refresh token")
		return
	}

	response.OK(c, output)
}

func (h *AuthHandler) Logout(c *gin.Context) {
	sessionVal, exists := c.Get("session_id")
	if !exists {
		response.BadRequest(c, "missing session identifier")
		return
	}

	session, ok := sessionVal.(string)
	if !ok || session == "" {
		response.BadRequest(c, "invalid session identifier")
		return
	}

	if err := h.usecase.Logout(c.Request.Context(), session); err != nil {
		response.Unauthorized(c, err.Error())
		return
	}

	response.OK(c, "logout successful")

}
