package user

import (
	"github.com/motixo/goat-api/internal/domain/repository"
	"github.com/motixo/goat-api/internal/domain/service"
)

type UserUseCase struct {
	userRepo       repository.UserRepository
	passwordHasher service.PasswordHasher
	logger         service.Logger
}

func NewUsecase(r repository.UserRepository, passwordHasher service.PasswordHasher, logger service.Logger) UseCase {
	return &UserUseCase{
		userRepo:       r,
		passwordHasher: passwordHasher,
		logger:         logger,
	}
}
