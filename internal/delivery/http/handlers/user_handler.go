package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/mot0x0/gopi/internal/delivery/http/dto"
	"github.com/mot0x0/gopi/internal/delivery/http/response"
	"github.com/mot0x0/gopi/internal/domain/usecase/user"
)

type UserHandler struct {
	userUC user.UserUseCase
}

func NewUserHandler(userUC user.UserUseCase) *UserHandler {
	return &UserHandler{userUC: userUC}
}

func (h *UserHandler) Register(c *gin.Context) {
	var req dto.CreateUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request payload")
		return
	}

	newUser, err := h.userUC.Register(c.Request.Context(), req.Email, req.Password)
	if err != nil {
		response.DomainError(c, err)
		return
	}

	userResponse := dto.UserResponse{
		ID:        newUser.ID,
		Email:     newUser.Email,
		Status:    newUser.Status.String(),
		CreatedAt: newUser.CreatedAt,
	}

	response.Created(c, userResponse)
}

func (h *UserHandler) Login(c *gin.Context) {
	var req dto.LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request payload")
		return
	}

	user, accessToken, refreshToken, err := h.userUC.Login(c.Request.Context(), req.Email, req.Password)
	if err != nil {
		response.DomainError(c, err)
		return
	}

	c.JSON(http.StatusOK, dto.LoginResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		User: dto.UserResponse{
			ID:        user.ID,
			Email:     user.Email,
			Status:    user.Status.String(),
			CreatedAt: user.CreatedAt,
		},
	})
}
