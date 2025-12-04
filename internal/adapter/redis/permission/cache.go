package permission

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/motixo/goth-api/internal/domain/entity"
	"github.com/redis/go-redis/v9"
)

type Cache struct {
	rdb *redis.Client
	ttl time.Duration
}

func NewCache(rdb *redis.Client, ttl time.Duration) *Cache {
	return &Cache{rdb: rdb, ttl: ttl}
}

func (c *Cache) key(roleID int8) string {
	return fmt.Sprintf("perm:%d", roleID)
}

func (c *Cache) Get(ctx context.Context, roleID int8) (*[]entity.Permission, error) {
	data, err := c.rdb.Get(ctx, c.key(roleID)).Bytes()
	if err == redis.Nil {
		return nil, nil // cache miss
	}
	if err != nil {
		return nil, err
	}

	var perms []entity.Permission
	if err := json.Unmarshal(data, &perms); err != nil {
		return nil, err
	}
	return &perms, nil
}

func (c *Cache) Set(ctx context.Context, roleID int8, perms *[]entity.Permission) error {
	b, err := json.Marshal(perms)
	if err != nil {
		return err
	}
	return c.rdb.Set(ctx, c.key(roleID), b, c.ttl).Err()
}

func (c *Cache) Delete(ctx context.Context, roleID int8) error {
	return c.rdb.Del(ctx, c.key(roleID)).Err()
}
