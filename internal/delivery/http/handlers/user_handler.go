package handlers

import "github.com/mot0x0/gopi/internal/domain/usecase/user"

type UserHandler struct {
	usecase user.UserUseCase
}

func NewUserHandler(usecase user.UserUseCase) *UserHandler {
	return &UserHandler{usecase: usecase}
}
