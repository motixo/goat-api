package auth

import (
	"github.com/mot0x0/goth-api/internal/config"
	"github.com/mot0x0/goth-api/internal/domain/service"
	"github.com/mot0x0/goth-api/internal/domain/usecase/session"
	"github.com/mot0x0/goth-api/internal/domain/usecase/user"
)

type AuthUseCase struct {
	userRepo       user.Repository
	sessionUC      session.UseCase
	ulidGen        *service.ULIDGenerator
	passwordHasher service.PasswordHasher
	logger         service.Logger
	jwtSecret      string
}

func NewUsecase(
	userRepo user.Repository,
	sessionUC session.UseCase,
	passwordHasher service.PasswordHasher,
	logger service.Logger,
	ulidGen *service.ULIDGenerator,
	cfg *config.Config,
) UseCase {
	return &AuthUseCase{
		userRepo:       userRepo,
		sessionUC:      sessionUC,
		passwordHasher: passwordHasher,
		logger:         logger,
		ulidGen:        ulidGen,
		jwtSecret:      cfg.JWTSecret,
	}
}
