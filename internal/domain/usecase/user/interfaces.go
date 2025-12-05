package user

import (
	"context"

	"github.com/motixo/goat-api/internal/domain/entity"
)

type UseCase interface {
	GetProfile(ctx context.Context, userID string) (*entity.User, error)
	GetUser(ctx context.Context, userID string) (*entity.User, error)
}
