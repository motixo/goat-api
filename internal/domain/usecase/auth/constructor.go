package auth

import (
	"github.com/mot0x0/gopi/internal/domain/repository"
	"github.com/mot0x0/gopi/internal/domain/usecase/jti"
)

type AuthUsecase struct {
	userRepo repository.UserRepository
	jtiUC    jti.JTIUseCase
}

func NewAuthUsecase(jtiUC jti.JTIUseCase, userRepo repository.UserRepository) AuthUseCase {
	return &AuthUsecase{
		userRepo: userRepo,
		jtiUC:    jtiUC,
	}
}
