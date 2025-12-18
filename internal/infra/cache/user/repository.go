package usercache

import (
	"context"

	"github.com/motixo/goat-api/internal/domain/errors"
	"github.com/motixo/goat-api/internal/domain/repository"
	"github.com/motixo/goat-api/internal/domain/service"
	"github.com/motixo/goat-api/internal/domain/valueobject"
)

type CachedRepository struct {
	dbRepo repository.UserRepository
	cache  *Cache
	logger service.Logger
}

func NewCachedRepository(
	dbRepo repository.UserRepository,
	cache *Cache,
	logger service.Logger,
) service.UserCacheService {
	return &CachedRepository{
		dbRepo: dbRepo,
		cache:  cache,
		logger: logger,
	}
}

func (c *CachedRepository) GetUserStatus(ctx context.Context, userID string) (valueobject.UserStatus, error) {
	if userCache, _ := c.cache.Get(ctx, userID); userCache != nil {
		return valueobject.UserStatus(userCache.Status), nil
	}

	user, err := c.dbRepo.FindByID(ctx, userID)
	if err != nil {
		return valueobject.StatusUnknown, err
	}
	if user == nil {
		return valueobject.StatusUnknown, errors.ErrUserNotFound
	}

	_ = c.cache.Set(ctx, userID, int8(user.Role), int8(user.Status))
	c.logger.Info("user cached successfully", "role", userID)
	return user.Status, nil
}

func (c *CachedRepository) GetUserRole(ctx context.Context, userID string) (valueobject.UserRole, error) {
	if userCache, _ := c.cache.Get(ctx, userID); userCache != nil {
		return valueobject.UserRole(userCache.Role), nil
	}

	user, err := c.dbRepo.FindByID(ctx, userID)
	if err != nil {
		return valueobject.RoleUnknown, err
	}
	if user == nil {
		return valueobject.RoleUnknown, errors.ErrUserNotFound
	}

	_ = c.cache.Set(ctx, userID, int8(user.Role), int8(user.Status))
	c.logger.Info("user cached successfully", "role", userID)
	return user.Role, nil
}

func (c *CachedRepository) ClearCache(ctx context.Context, userID string) error {
	if err := c.cache.Delete(ctx, userID); err != nil {
		c.logger.Info("clear user cache failed", "role", userID, "error", err)
		return err
	}
	c.logger.Info("user cache cleared successfully", "role", userID)
	return nil
}
