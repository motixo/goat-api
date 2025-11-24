package handlers

import (
	"github.com/gin-gonic/gin"
	"github.com/mot0x0/gopi/internal/delivery/http/response"
	"github.com/mot0x0/gopi/internal/domain/usecase/user"
)

type AuthHandler struct {
	usecase user.UserUseCase
}

func NewAuthHandler(usecase user.UserUseCase) *AuthHandler {
	return &AuthHandler{usecase: usecase}
}

func (h *AuthHandler) Register(c *gin.Context) {
	var input user.RegisterInput
	if err := c.ShouldBindJSON(&input); err != nil {
		response.BadRequest(c, "Invalid request payload")
		return
	}

	output, err := h.usecase.Register(c.Request.Context(), input)
	if err != nil {
		response.DomainError(c, err)
		return
	}

	response.Created(c, output)
}

func (h *AuthHandler) Login(c *gin.Context) {
	var input user.LoginInput
	if err := c.ShouldBindJSON(&input); err != nil {
		response.BadRequest(c, "Invalid request payload")
		return
	}

	output, err := h.usecase.Login(c.Request.Context(), input)
	if err != nil {
		response.DomainError(c, err)
		return
	}

	response.OK(c, output)
}

func (h *AuthHandler) Refresh(c *gin.Context) {
	var input user.RefreshInput
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
