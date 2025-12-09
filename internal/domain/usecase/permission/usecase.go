package permission

import (
	"context"

	"github.com/motixo/goat-api/internal/domain/entity"
	"github.com/motixo/goat-api/internal/domain/repository"
	"github.com/motixo/goat-api/internal/domain/valueobject"
	"github.com/motixo/goat-api/internal/infra/logger"
)

type PermissionUseCase struct {
	permissionRepo repository.PermissionRepository
	logger         logger.Logger
}

func NewUsecase(
	p repository.PermissionRepository,
	logger logger.Logger,
) UseCase {
	return &PermissionUseCase{
		permissionRepo: p,
		logger:         logger,
	}
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
