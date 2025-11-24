package user

import (
	"context"

	"github.com/mot0x0/gopi/internal/domain/entity"
)

type UserUseCase interface {
	Register(ctx context.Context, email, password string) (*entity.User, error)
	Login(ctx context.Context, email, password string) (*entity.User, string, string, error)
	GetProfile(ctx context.Context, userID string) (*entity.User, error)
	ValidateToken(ctx context.Context, token string) (string, error)
}
