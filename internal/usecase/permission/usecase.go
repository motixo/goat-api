package permission

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/motixo/goat-api/internal/domain/entity"
	"github.com/motixo/goat-api/internal/domain/event"
	"github.com/motixo/goat-api/internal/domain/repository"
	"github.com/motixo/goat-api/internal/domain/valueobject"
	"github.com/motixo/goat-api/internal/pkg"
)

type PermissionUseCase struct {
	permissionRepo repository.PermissionRepository
	publisher      event.Publisher
	logger         pkg.Logger
}

func NewUsecase(
	p repository.PermissionRepository,
	publisher event.Publisher,
	logger pkg.Logger,
) UseCase {
	return &PermissionUseCase{
		permissionRepo: p,
		logger:         logger,
		publisher:      publisher,
	}
}

func (us *PermissionUseCase) Create(ctx context.Context, input CreateInput) (PermissionOutput, error) {
	us.logger.Info("create permission attempt", "role", input.Role.String(), "action", input.Action)
	perm := &entity.Permission{
		ID:        uuid.New().String(),
		Role:      input.Role,
		Action:    input.Action,
		CreatedAt: time.Now().UTC(),
	}
	if err := us.permissionRepo.Create(ctx, perm); err != nil {
		us.logger.Error("failed to create permission", "role", input.Role.String(), "action", input.Action, "error", err)
		return PermissionOutput{}, err
	}

	us.publisher.Publish(ctx, event.PermissionUpdatedEvent{
		Role: input.Role,
	})

	us.logger.Info("permission created successfully", "role", input.Role.String(), "action", input.Action)
	return PermissionOutput{
		ID:        perm.ID,
		Role:      perm.Role.String(),
		Action:    perm.Action.String(),
		CreatedAt: perm.CreatedAt,
	}, nil
}

func (us *PermissionUseCase) GetPermissions(ctx context.Context, offset, limit int) ([]PermissionOutput, int64, error) {
	us.logger.Info("fetching all permissions")
	perms, total, err := us.permissionRepo.List(ctx, offset, limit)
	if err != nil {
		us.logger.Error("failed to fetch permissions", "error", err)
		return nil, 0, err
	}

	response := make([]PermissionOutput, 0, len(perms))
	for _, perm := range perms {
		r := PermissionOutput{
			ID:        perm.ID,
			Role:      perm.Role.String(),
			Action:    perm.Action.String(),
			CreatedAt: perm.CreatedAt,
		}
		response = append(response, r)
	}
	us.logger.Info("all permissions fetched successfully")
	return response, total, nil
}

func (us *PermissionUseCase) GetPermissionsByRole(ctx context.Context, role valueobject.UserRole) ([]PermissionOutput, error) {
	us.logger.Info("fetching permissions for role", "role_id", role.String())
	perms, err := us.permissionRepo.GetByRoleID(ctx, role)
	if err != nil {
		us.logger.Error("failed to fetch permissions", "role_id", role.String(), "error", err)
		return nil, err
	}

	response := make([]PermissionOutput, 0, len(perms))
	for _, perm := range perms {
		r := PermissionOutput{
			ID:        perm.ID,
			Role:      perm.Role.String(),
			Action:    perm.Action.String(),
			CreatedAt: perm.CreatedAt,
		}
		response = append(response, r)
	}
	us.logger.Info("permissions fetched successfully", "role_id", role.String())
	return response, nil
}

func (us *PermissionUseCase) Delete(ctx context.Context, permissionID string) error {
	us.logger.Info("delete permission attempt", "permission_id", permissionID)
	roleID, err := us.permissionRepo.Delete(ctx, permissionID)
	if err != nil {
		us.logger.Error("failed to create permission", "permission_id", permissionID, "error", err)
		return err
	}
	us.publisher.Publish(ctx, event.PermissionUpdatedEvent{
		Role: valueobject.UserRole(roleID),
	})

	us.logger.Info("permission deleted successfully", "permission_id", permissionID)
	return nil
}
