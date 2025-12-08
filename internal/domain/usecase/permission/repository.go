package permission

import (
	"context"

	"github.com/motixo/goat-api/internal/domain/entity"
	"github.com/motixo/goat-api/internal/domain/valueobject"
)

type CreateInput struct {
	RoleID string
	Action string
}

func (us *PermissionUseCase) Create(ctx context.Context, input CreateInput) error {
	return nil
}

func (us *PermissionUseCase) GetPermissionsByRole(ctx context.Context, roleID valueobject.UserRole) ([]*entity.Permission, error) {
	us.logger.Info("fetching permissions for role", "role_id", roleID)
	perms, err := us.permissionRepo.GetByRoleID(ctx, int8(roleID))
	if err != nil {
		us.logger.Error("failed to fetch permissions", "role_id", roleID, "Error", err)
		return nil, err
	}
	us.logger.Info("permissions fetched successfully", "role_id", roleID)
	return perms, nil
}
