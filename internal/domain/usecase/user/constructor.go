package user

import "github.com/mot0x0/gopi/internal/domain/service"

type UserUseCase struct {
	userRepo        Repository
	passwordService *service.PasswordService
}

func NewUserUsecase(r Repository, passwordSvc *service.PasswordService) UseCase {
	return &UserUseCase{
		userRepo:        r,
		passwordService: passwordSvc,
	}
}
