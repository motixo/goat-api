package repository

import (
	"context"

	"github.com/motixo/goat-api/internal/domain/entity"
	"github.com/motixo/goat-api/internal/domain/valueobject"
)

type UserRepository interface {
	Create(ctx context.Context, u *entity.User) error
	ExistsByID(ctx context.Context, id string) (bool, error)
	FindByID(ctx context.Context, id string) (*entity.User, error)
	FindByEmail(ctx context.Context, email string) (*entity.User, error)
	GetCredentialVersion(ctx context.Context, id string) (int64, error)
	UpdatePassword(ctx context.Context, id string, password valueobject.Password) (int64, error)
	Update(ctx context.Context, u *entity.User) error
	Delete(ctx context.Context, userID string) error
	List(ctx context.Context, offset, limit int, filters UserListFilter) ([]*entity.User, int64, error)
}
