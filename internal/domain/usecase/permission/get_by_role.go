package permission

import (
	"context"

	"github.com/motixo/goth-api/internal/domain/entity"
	"github.com/motixo/goth-api/internal/domain/valueobject"
)

func (p *PermissionUseCase) GetPermissionsByRole(ctx context.Context, roleID valueobject.UserRole) (*[]entity.Permission, error) {
	p.logger.Info("fetching permissions for role", "role_id", roleID)
	perms, err := p.permissionRepo.GetByRoleID(ctx, int8(roleID))
	if err != nil {
		p.logger.Error("failed to fetch permissions", "role_id", roleID, "Error", err)
		return nil, err
	}
	p.logger.Info("permissions fetched successfully", "role_id", roleID)
	return perms, nil
}
