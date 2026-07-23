package permcache

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/motixo/goat-api/internal/domain/entity"
	"github.com/motixo/goat-api/internal/domain/valueobject"
	"github.com/motixo/goat-api/internal/pkg"
	"github.com/redis/go-redis/v9"
)

type Cache struct {
	rdb *redis.Client
	ttl time.Duration
}

func NewCache(rdb *redis.Client) *Cache {
	return &Cache{
		rdb: rdb,
		ttl: 24 * time.Hour,
	}
}

func (c *Cache) Get(ctx context.Context, roleID int8) ([]*entity.Permission, error) {
	role, err := permissionCacheRole(roleID)
	if err != nil {
		return nil, err
	}

	data, err := c.rdb.Get(ctx, pkg.RedisKey("perm", "role", roleID)).Bytes()
	if errors.Is(err, redis.Nil) {
		return nil, nil // cache miss
	}
	if err != nil {
		return nil, fmt.Errorf("read permission cache for role %q: %w", role.String(), err)
	}

	var records []permissionCacheRecord
	if err := json.Unmarshal(data, &records); err != nil {
		return nil, fmt.Errorf("decode permission cache for role %q: %w", role.String(), err)
	}
	permissions, err := permissionCacheRecordsToDomain(records, role)
	if err != nil {
		return nil, fmt.Errorf("decode permission cache for role %q: %w", role.String(), err)
	}
	return permissions, nil
}

func (c *Cache) Set(ctx context.Context, roleID int8, perms []*entity.Permission) error {
	role, err := permissionCacheRole(roleID)
	if err != nil {
		return err
	}
	records, err := permissionCacheRecordsFromDomain(perms, role)
	if err != nil {
		return fmt.Errorf("encode permission cache for role %q: %w", role.String(), err)
	}
	b, err := json.Marshal(records)
	if err != nil {
		return fmt.Errorf("encode permission cache for role %q: %w", role.String(), err)
	}
	if err := c.rdb.Set(ctx, pkg.RedisKey("perm", "role", roleID), b, c.ttl).Err(); err != nil {
		return fmt.Errorf("write permission cache for role %q: %w", role.String(), err)
	}
	return nil
}

func (c *Cache) Delete(ctx context.Context, roleID int8) error {
	return c.rdb.Del(ctx, pkg.RedisKey("perm", "role", roleID)).Err()
}

// fallback
func (c *Cache) DeleteAll(ctx context.Context) error {
	userRoles := valueobject.AllRoles()
	for _, role := range userRoles {
		if err := c.rdb.Del(ctx, pkg.RedisKey("perm", "role", int8(role))).Err(); err != nil {
			return err
		}
	}
	return nil
}
