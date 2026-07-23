package permcache

import (
	"context"

	"github.com/motixo/goat-api/internal/domain/entity"
	"github.com/motixo/goat-api/internal/domain/repository"
	"github.com/motixo/goat-api/internal/domain/service"
	"github.com/motixo/goat-api/internal/domain/valueobject"
	"github.com/motixo/goat-api/internal/pkg"
)

type CachedRepository struct {
	dbRepo repository.PermissionRepository
	cache  *Cache
	logger pkg.Logger
}

func NewCachedRepository(
	dbRepo repository.PermissionRepository,
	cache *Cache,
	logger pkg.Logger,
) service.PermCacheService {
	return &CachedRepository{
		dbRepo: dbRepo,
		cache:  cache,
		logger: logger,
	}
}

func (c *CachedRepository) GetRolePermissions(ctx context.Context, role valueobject.UserRole) ([]*entity.Permission, error) {
	roleID := int8(role)
	perms, cacheErr := c.cache.Get(ctx, roleID)
	if cacheErr != nil {
		c.logger.Warn(
			"read permission cache failed; falling back to database",
			"role", role.String(),
			"error", cacheErr,
		)
	} else if perms != nil {
		return perms, nil
	}

	perms, err := c.dbRepo.GetByRoleID(ctx, role)
	if err != nil {
		return nil, err
	}

	if err := c.cache.Set(ctx, roleID, perms); err != nil {
		c.logger.Warn("write permission cache failed", "role", role.String(), "error", err)
	} else {
		c.logger.Info("permission cached successfully", "role", role.String())
	}
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
