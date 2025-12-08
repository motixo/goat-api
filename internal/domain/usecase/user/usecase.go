package user

import (
	"github.com/motixo/goat-api/internal/domain/repository"
	"github.com/motixo/goat-api/internal/domain/service"
	"github.com/motixo/goat-api/internal/infrastructure/logger"
)

type UserUseCase struct {
	userRepo       repository.UserRepository
	passwordHasher service.PasswordHasher
	sessionRepo    repository.SessionRepository
	logger         logger.Logger
}

func NewUsecase(
	r repository.UserRepository,
	passwordHasher service.PasswordHasher,
	logger logger.Logger,
	sessionRepo repository.SessionRepository,
) UseCase {
	return &UserUseCase{
		userRepo:       r,
		passwordHasher: passwordHasher,
		sessionRepo:    sessionRepo,
		logger:         logger,
	}
}
