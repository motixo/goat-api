package permission

import (
	"context"

	"github.com/motixo/goth-api/internal/domain/entity"
	"github.com/motixo/goth-api/internal/domain/valueobject"
)

func (us *PermissionUseCase) GetPermissionsByRole(ctx context.Context, roleID valueobject.UserRole) (*[]entity.Permission, error) {
	us.logger.Info("fetching permissions for role", "role_id", roleID)
	perms, err := us.permissionRepo.GetByRoleID(ctx, int8(roleID))
	if err != nil {
		us.logger.Error("failed to fetch permissions", "role_id", roleID, "Error", err)
		return nil, err
	}
	us.logger.Info("permissions fetched successfully", "role_id", roleID)
	return perms, nil
}
