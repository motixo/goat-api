package permission

import (
	"github.com/motixo/goat-api/internal/domain/repository"
	"github.com/motixo/goat-api/internal/infrastructure/logger"
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
