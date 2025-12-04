package permission

import (
	"context"

	"github.com/motixo/goth-api/internal/adapter/postgres/permission"
	permissionCache "github.com/motixo/goth-api/internal/adapter/redis/permission"
	"github.com/motixo/goth-api/internal/domain/entity"
	"github.com/motixo/goth-api/internal/domain/service"
	permissionUC "github.com/motixo/goth-api/internal/domain/usecase/permission"
)

type CachedRepository struct {
	dbRepo *permission.Repository
	cache  *permissionCache.Cache
	logger service.Logger
}

func NewCachedRepository(dbRepo *permission.Repository, cache *permissionCache.Cache, logger service.Logger) permissionUC.Repository {
	return &CachedRepository{dbRepo: dbRepo, cache: cache, logger: logger}
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
