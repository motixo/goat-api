package auth

import (
	"github.com/mot0x0/goth-api/internal/config"
	"github.com/mot0x0/goth-api/internal/domain/service"
	"github.com/mot0x0/goth-api/internal/domain/usecase/session"
	"github.com/mot0x0/goth-api/internal/domain/usecase/user"
)

type AuthUseCase struct {
	userRepo        user.Repository
	sessionUC       session.UseCase
	ulidGen         *service.ULIDGenerator
	passwordService *service.PasswordService
	jwtSecret       string
}

func NewUsecase(
	sessionUC session.UseCase,
	userRepo user.Repository,
	passwordSvc *service.PasswordService,
	ulidGen *service.ULIDGenerator,
	cfg *config.Config,
) UseCase {
	return &AuthUseCase{
		userRepo:        userRepo,
		sessionUC:       sessionUC,
		passwordService: passwordSvc,
		ulidGen:         ulidGen,
		jwtSecret:       cfg.JWTSecret,
	}
}
