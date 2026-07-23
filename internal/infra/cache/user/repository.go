package usercache

import (
	"context"
	"fmt"

	"github.com/motixo/goat-api/internal/domain/errors"
	"github.com/motixo/goat-api/internal/domain/repository"
	"github.com/motixo/goat-api/internal/domain/service"
	"github.com/motixo/goat-api/internal/domain/valueobject"
	"github.com/motixo/goat-api/internal/pkg"
)

type CachedRepository struct {
	dbRepo repository.UserRepository
	cache  *Cache
	logger pkg.Logger
}

func NewCachedRepository(
	dbRepo repository.UserRepository,
	cache *Cache,
	logger pkg.Logger,
) service.UserCacheService {
	return &CachedRepository{
		dbRepo: dbRepo,
		cache:  cache,
		logger: logger,
	}
}

func (c *CachedRepository) GetUserStatus(ctx context.Context, userID string) (valueobject.UserStatus, error) {
	authorization, err := c.getUserAuthorization(ctx, userID)
	if err != nil {
		return valueobject.StatusUnknown, err
	}
	return authorization.status, nil
}

func (c *CachedRepository) GetUserRole(ctx context.Context, userID string) (valueobject.UserRole, error) {
	authorization, err := c.getUserAuthorization(ctx, userID)
	if err != nil {
		return valueobject.RoleUnknown, err
	}
	return authorization.role, nil
}

func (c *CachedRepository) getUserAuthorization(ctx context.Context, userID string) (*userAuthorization, error) {
	authorization, cacheErr := c.cache.Get(ctx, userID)
	if cacheErr != nil {
		c.logger.Warn(
			"read user authorization cache failed; falling back to database",
			"user_id", userID,
			"error", cacheErr,
		)
	} else if authorization != nil {
		return authorization, nil
	}

	user, err := c.dbRepo.FindByID(ctx, userID)
	if err != nil {
		return nil, err
	}
	if user == nil {
		return nil, errors.ErrUserNotFound
	}

	authorization, err = userAuthorizationFromDomain(user, userID)
	if err != nil {
		return nil, fmt.Errorf("validate authoritative user authorization: %w", err)
	}

	if err := c.cache.Set(ctx, userID, user); err != nil {
		c.logger.Warn("write user authorization cache failed", "user_id", userID, "error", err)
	} else {
		c.logger.Info("user authorization cached successfully", "user_id", userID)
	}
	return authorization, nil
}

func (c *CachedRepository) ClearCache(ctx context.Context, userID string) error {
	if err := c.cache.Delete(ctx, userID); err != nil {
		c.logger.Error("clear user authorization cache failed", "user_id", userID, "error", err)
		return err
	}
	c.logger.Info("user authorization cache cleared successfully", "user_id", userID)
	return nil
}
