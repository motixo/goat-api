package auth

import (
	"github.com/mot0x0/gopi/internal/config"
	"github.com/mot0x0/gopi/internal/domain/service"
	"github.com/mot0x0/gopi/internal/domain/usecase/jti"
	"github.com/mot0x0/gopi/internal/domain/usecase/session"
	"github.com/mot0x0/gopi/internal/domain/usecase/user"
)

type AuthUseCase struct {
	userRepo        user.Repository
	jtiUC           jti.UseCase
	sessionUC       session.UseCase
	ulidGen         *service.ULIDGenerator
	passwordService *service.PasswordService
	jwtSecret       string
}

func NewUsecase(
	jtiUC jti.UseCase,
	sessionUC session.UseCase,
	userRepo user.Repository,
	passwordSvc *service.PasswordService,
	ulidGen *service.ULIDGenerator,
	cfg *config.Config,
) UseCase {
	return &AuthUseCase{
		userRepo:        userRepo,
		jtiUC:           jtiUC,
		sessionUC:       sessionUC,
		passwordService: passwordSvc,
		ulidGen:         ulidGen,
		jwtSecret:       cfg.JWTSecret,
	}
}
