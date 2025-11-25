package user

import (
	"github.com/mot0x0/gopi/internal/domain/repository"
	"github.com/mot0x0/gopi/internal/domain/usecase/jti"
)

type UserUsecase struct {
	userRepo repository.UserRepository
	jtiUC    jti.JTIUseCase
}

func NewUserUsecase(r repository.UserRepository, jtiUC jti.JTIUseCase) UserUseCase {
	return &UserUsecase{
		userRepo: r,
		jtiUC:    jtiUC,
	}
}
