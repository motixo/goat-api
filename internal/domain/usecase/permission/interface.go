package permission

import (
	"context"

	"github.com/motixo/goat-api/internal/domain/entity"
	"github.com/motixo/goat-api/internal/domain/valueobject"
)

type UseCase interface {
	Create(ctx context.Context, input CreateInput) (*entity.Permission, error)
	GetPermissions(ctx context.Context, offset, limit int) ([]*PermissionResponse, int64, error)
	GetPermissionsByRole(ctx context.Context, roleID valueobject.UserRole) ([]*entity.Permission, error)
	Delete(ctx context.Context, permissionID string) error
}
