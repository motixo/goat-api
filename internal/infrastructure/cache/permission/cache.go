package permission

import (
	"context"
	"encoding/json"
	"time"

	"github.com/motixo/goat-api/internal/domain/entity"
	"github.com/motixo/goat-api/internal/infrastructure/helper"
	"github.com/redis/go-redis/v9"
)

type Cache struct {
	rdb *redis.Client
	ttl time.Duration
}

func NewCache(rdb *redis.Client, ttl time.Duration) *Cache {
	return &Cache{rdb: rdb, ttl: ttl}
}

func (c *Cache) Get(ctx context.Context, roleID int8) ([]*entity.Permission, error) {
	data, err := c.rdb.Get(ctx, helper.Key("perm", "role", roleID)).Bytes()
	if err == redis.Nil {
		return nil, nil // cache miss
	}
	if err != nil {
		return nil, err
	}

	var perms []*entity.Permission
	if err := json.Unmarshal(data, &perms); err != nil {
		return nil, err
	}
	return perms, nil
}

func (c *Cache) Set(ctx context.Context, roleID int8, perms []*entity.Permission) error {
	b, err := json.Marshal(perms)
	if err != nil {
		return err
	}
	return c.rdb.Set(ctx, helper.Key("perm", "role", roleID), b, c.ttl).Err()
}

func (c *Cache) Delete(ctx context.Context, roleID int8) error {
	return c.rdb.Del(ctx, helper.Key("perm", "role", roleID)).Err()
}
