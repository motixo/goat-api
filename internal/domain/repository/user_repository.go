package repository

import (
	"context"

	"github.com/mot0x0/gopi/internal/domain/entity"
)

type UserRepository interface {
	Create(ctx context.Context, u *entity.User) error
	FindByID(ctx context.Context, id string) (*entity.User, error)
	FindByEmail(ctx context.Context, email string) (*entity.User, error)
}
