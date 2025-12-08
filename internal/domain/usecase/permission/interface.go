package permission

import (
	"context"

	"github.com/motixo/goat-api/internal/domain/entity"
	"github.com/motixo/goat-api/internal/domain/valueobject"
)

type UseCase interface {
	Create(ctx context.Context, input CreateInput) error
	GetPermissionsByRole(ctx context.Context, roleID valueobject.UserRole) ([]*entity.Permission, error)
}
