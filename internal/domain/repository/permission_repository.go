package repository

import (
	"context"

	"github.com/motixo/goat-api/internal/domain/entity"
	"github.com/motixo/goat-api/internal/domain/valueobject"
)

type PermissionRepository interface {
	Create(ctx context.Context, p *entity.Permission) error
	List(ctx context.Context, offset, limit int) ([]*entity.Permission, int64, error)
	GetByRoleID(ctx context.Context, roleID valueobject.UserRole) ([]*entity.Permission, error)
	Delete(ctx context.Context, permissionID string) error
}
