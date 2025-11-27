package user

import (
	"context"

	"github.com/mot0x0/gopi/internal/domain/entity"
)

type UserUseCase interface {
	Register(ctx context.Context, input RegisterInput) (RegisterOutput, error)
	GetProfile(ctx context.Context, userID string) (*entity.User, error)
}
