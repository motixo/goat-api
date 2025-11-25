package user

import (
	"context"

	"github.com/mot0x0/gopi/internal/domain/entity"
)

type UserUseCase interface {
	Register(ctx context.Context, input RegisterInput) (RegisterOutput, error)
	Login(ctx context.Context, input LoginInput) (LoginOutput, error)
	Refresh(ctx context.Context, input RefreshInput) (RefreshOutput, error)
	Logout(ctx context.Context, input LogoutInput) error
	GetProfile(ctx context.Context, userID string) (*entity.User, error)
}
