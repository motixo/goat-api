package auth

import (
	"github.com/mot0x0/gopi/internal/domain/service"
	"github.com/mot0x0/gopi/internal/domain/usecase/jti"
	"github.com/mot0x0/gopi/internal/domain/usecase/user"
)

type AuthUseCase struct {
	userRepo        user.Repository
	jtiUC           jti.UseCase
	passwordService *service.PasswordService
	jwtSecret       string
}

func NewAuthUsecase(jtiUC jti.UseCase, userRepo user.Repository, passwordSvc *service.PasswordService, jwtSecret string) UseCase {
	return &AuthUseCase{
		userRepo:        userRepo,
		jtiUC:           jtiUC,
		passwordService: passwordSvc,
		jwtSecret:       jwtSecret,
	}
}
