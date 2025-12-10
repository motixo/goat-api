package permission

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/motixo/goat-api/internal/domain/entity"
	"github.com/motixo/goat-api/internal/domain/repository"
	"github.com/motixo/goat-api/internal/domain/service"
	"github.com/motixo/goat-api/internal/domain/valueobject"
	"github.com/motixo/goat-api/internal/infra/logger"
)

type PermissionUseCase struct {
	permissionRepo repository.PermissionRepository
	ulidGen        service.IDGenerator
	logger         logger.Logger
}

func NewUsecase(
	p repository.PermissionRepository,
	ulidGen service.IDGenerator,
	logger logger.Logger,
) UseCase {
	return &PermissionUseCase{
		permissionRepo: p,
		logger:         logger,
		ulidGen:        ulidGen,
	}
}

func (us *PermissionUseCase) Create(ctx context.Context, input CreateInput) (*entity.Permission, error) {
	us.logger.Info("create permission attempt", "role_id", input.RoleID, "action", input.Action)
	perm := entity.Permission{
		ID:        uuid.New().String(),
		RoleID:    input.RoleID,
		Action:    input.Action,
		CreatedAt: time.Now().UTC(),
	}
	if err := us.permissionRepo.Create(ctx, &perm); err != nil {
		us.logger.Error("failed to create permission", "role_id", input.RoleID, "action", input.Action, "error", err)
		return nil, err
	}
	us.logger.Info("permission created successfully", "role_id", input.RoleID, "action", input.Action)
	return &perm, nil
}

func (us *PermissionUseCase) GetPermissions(ctx context.Context, offset, limit int) ([]*PermissionResponse, int64, error) {
	us.logger.Info("fetching all permissions")
	perms, total, err := us.permissionRepo.GetAll(ctx, offset, limit)
	if err != nil {
		us.logger.Error("failed to fetch permissions", "error", err)
		return nil, 0, err
	}

	response := make([]*PermissionResponse, 0, len(perms))
	for _, perm := range perms {
		r := &PermissionResponse{
			ID:        perm.ID,
			Role:      valueobject.UserRole(perm.RoleID).String(),
			Action:    perm.Action,
			CreatedAt: perm.CreatedAt,
		}
		response = append(response, r)
	}
	us.logger.Info("all permissions fetched successfully")
	return response, total, nil
}

func (us *PermissionUseCase) GetPermissionsByRole(ctx context.Context, roleID valueobject.UserRole) ([]*entity.Permission, error) {
	us.logger.Info("fetching permissions for role", "role_id", roleID)
	perms, err := us.permissionRepo.GetByRoleID(ctx, int8(roleID))
	if err != nil {
		us.logger.Error("failed to fetch permissions", "role_id", roleID, "error", err)
		return nil, err
	}
	us.logger.Info("permissions fetched successfully", "role_id", roleID)
	return perms, nil
}

func (us *PermissionUseCase) Delete(ctx context.Context, permissionID string) error {
	us.logger.Info("delete permission attempt", "permission_id", permissionID)
	if _, err := us.permissionRepo.Delete(ctx, permissionID); err != nil {
		us.logger.Error("failed to create permission", "permission_id", permissionID, "error", err)
		return err
	}
	return nil
}
