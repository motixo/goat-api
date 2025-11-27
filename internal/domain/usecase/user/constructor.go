package user

import (
	"github.com/mot0x0/gopi/internal/domain/repository"
)

type UserUsecase struct {
	userRepo repository.UserRepository
}

func NewUserUsecase(r repository.UserRepository) UserUseCase {
	return &UserUsecase{
		userRepo: r,
	}
}
