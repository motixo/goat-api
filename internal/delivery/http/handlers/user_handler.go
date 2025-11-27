package handlers

import (
	"github.com/gin-gonic/gin"
	"github.com/mot0x0/gopi/internal/delivery/http/response"
	"github.com/mot0x0/gopi/internal/domain/usecase/user"
)

type UserHandler struct {
	usecase user.UserUseCase
}

func NewUserHandler(usecase user.UserUseCase) *UserHandler {
	return &UserHandler{usecase: usecase}
}

func (h *UserHandler) Register(c *gin.Context) {
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
