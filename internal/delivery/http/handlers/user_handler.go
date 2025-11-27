package handlers

import (
	"github.com/mot0x0/gopi/internal/domain/usecase/user"
)

type UserHandler struct {
	usecase user.UseCase
}

func NewUserHandler(usecase user.UseCase) *UserHandler {
	return &UserHandler{usecase: usecase}
}
