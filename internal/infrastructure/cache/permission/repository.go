package permission

import (
	"context"

	"github.com/motixo/goat-api/internal/domain/entity"
	"github.com/motixo/goat-api/internal/domain/repository"
	"github.com/motixo/goat-api/internal/infrastructure/logger"
)

type CachedRepository struct {
	dbRepo repository.PermissionRepository
	cache  *Cache
	logger logger.Logger
}

func NewCachedRepository(
	dbRepo repository.PermissionRepository,
	cache *Cache,
	logger logger.Logger,
) repository.PermissionRepository {
	return &CachedRepository{
		dbRepo: dbRepo,
		cache:  cache,
		logger: logger,
	}
}

func (c *CachedRepository) GetByRoleID(ctx context.Context, roleID int8) (*[]entity.Permission, error) {
	if perms, _ := c.cache.Get(ctx, roleID); perms != nil {
		return perms, nil
	}

	perms, err := c.dbRepo.GetByRoleID(ctx, roleID)
	if err != nil {
		return nil, err
	}

	_ = c.cache.Set(ctx, roleID, perms)

	return perms, nil
}

func (c *CachedRepository) Create(ctx context.Context, p *entity.Permission) error {
	err := c.dbRepo.Create(ctx, p)
	if err != nil {
		return err
	}

	_ = c.cache.Delete(ctx, int8(p.RoleID))

	return nil
}
