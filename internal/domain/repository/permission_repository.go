package repository

import (
	"context"

	"github.com/motixo/goat-api/internal/domain/entity"
)

type PermissionRepository interface {
	Create(ctx context.Context, p *entity.Permission) error
	GetAll(ctx context.Context, offset, limit int) ([]*entity.Permission, int64, error)
	GetByRoleID(ctx context.Context, roleID int8) ([]*entity.Permission, error)
	Delete(ctx context.Context, permissionID string) (*int8, error)
}
