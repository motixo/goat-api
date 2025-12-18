package permcache

import (
	"context"

	"github.com/motixo/goat-api/internal/domain/entity"
	"github.com/motixo/goat-api/internal/domain/repository"
	"github.com/motixo/goat-api/internal/domain/service"
	"github.com/motixo/goat-api/internal/domain/valueobject"
)

type CachedRepository struct {
	dbRepo repository.PermissionRepository
	cache  *Cache
	logger service.Logger
}

func NewCachedRepository(
	dbRepo repository.PermissionRepository,
	cache *Cache,
	logger service.Logger,
) service.PermCacheService {
	return &CachedRepository{
		dbRepo: dbRepo,
		cache:  cache,
		logger: logger,
	}
}

func (c *CachedRepository) GetRolePermissions(ctx context.Context, role valueobject.UserRole) ([]*entity.Permission, error) {
	roleID := int8(role)
	if perms, _ := c.cache.Get(ctx, roleID); perms != nil {
		return perms, nil
	}

	perms, err := c.dbRepo.GetByRoleID(ctx, role)
	if err != nil {
		return nil, err
	}

	_ = c.cache.Set(ctx, roleID, perms)
	c.logger.Info("permission cached successfully", "role", role.String())
	return perms, nil
}

func (c *CachedRepository) ClearCache(ctx context.Context, role valueobject.UserRole) error {
	roleID := int8(role)
	if err := c.cache.Delete(ctx, roleID); err != nil {
		c.logger.Error("clear permission cache failed", "role", role.String(), "error", err)
		return err
	}
	c.logger.Info("permission cache cleared successfully", "role", role.String())
	return nil
}
