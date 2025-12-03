package permission

import (
	"github.com/motixo/goth-api/internal/domain/service"
)

type PermissionUseCase struct {
	permissionRepo Repository
	logger         service.Logger
}

func NewUsecase(
	p Repository,
	logger service.Logger,
) UseCase {
	return &PermissionUseCase{
		permissionRepo: p,
		logger:         logger,
	}
}
