package Role

import (
	"context"

	"github.com/motixo/goat-api/internal/domain/repository"
	"github.com/motixo/goat-api/internal/infrastructure/logger"
)

type CachedRepository struct {
	dbRepo repository.UserRepository
	cache  *Cache
	logger logger.Logger
}

func NewCachedRepository(
	dbRepo repository.UserRepository,
	cache *Cache,
	logger logger.Logger,
) repository.RoleRepository {
	return &CachedRepository{
		dbRepo: dbRepo,
		cache:  cache,
		logger: logger,
	}
}

func (c *CachedRepository) GetByUserID(ctx context.Context, userID string) (int8, error) {
	if userRole, _ := c.cache.Get(ctx, userID); userRole != -1 {
		return userRole, nil
	}

	user, err := c.dbRepo.FindByID(ctx, userID)
	if err != nil {
		return -1, err
	}

	_ = c.cache.Set(ctx, userID, int8(user.Role))

	return int8(user.Role), nil
}

// func (c *RoleCachedRepository) Create(ctx context.Context, p *entity.Permission) error {
// 	err := c.dbRepo.Create(ctx, p)
// 	if err != nil {
// 		return err
// 	}

// 	_ = c.cache.Delete(ctx, int8(p.RoleID))

// 	return nil
// }
