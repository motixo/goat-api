package handlers

import (
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/mot0x0/gopi/internal/delivery/http/response"
	"github.com/mot0x0/gopi/internal/domain/usecase/auth"
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

	output, err := h.usecase.Refresh(c.Request.Context(), input)
	if err != nil {
		response.Unauthorized(c, "invalid refresh token")
		return
	}

	response.OK(c, output)
}

func (h *AuthHandler) Logout(c *gin.Context) {
	authHeader := c.GetHeader("Authorization")
	if authHeader == "" {
		response.BadRequest(c, "Authorization header is required")
		return
	}

	tokenStr := strings.TrimPrefix(authHeader, "Bearer ")
	if tokenStr == authHeader {
		response.BadRequest(c, "Invalid Authorization header format")
		return
	}

	if err := h.usecase.Logout(c.Request.Context(), tokenStr); err != nil {
		response.Unauthorized(c, err.Error())
		return
	}

	response.OK(c, "logout successful")
}
