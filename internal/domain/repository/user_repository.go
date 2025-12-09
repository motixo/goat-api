package repository

import (
	"context"

	"github.com/motixo/goat-api/internal/domain/entity"
	"github.com/motixo/goat-api/internal/domain/repository/dto"
)

type UserRepository interface {
	Create(ctx context.Context, u *entity.User) error
	FindByID(ctx context.Context, id string) (*entity.User, error)
	FindByEmail(ctx context.Context, email string) (*entity.User, error)
	Update(ctx context.Context, userID string, u dto.UserUpdate) error
	Delete(ctx context.Context, userID string) error
	List(ctx context.Context, offset, limit int) ([]*entity.User, int64, error)
}
