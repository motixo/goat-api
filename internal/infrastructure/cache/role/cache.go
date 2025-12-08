package Role

import (
	"context"
	"strconv"
	"time"

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

func (c *Cache) Get(ctx context.Context, userID string) (int8, error) {
	key := helper.Key("role", "user", userID)
	val, err := c.rdb.Get(ctx, key).Result()
	if err == redis.Nil {
		return -1, nil // cache miss
	}
	if err != nil {
		return -1, err
	}

	i64, err := strconv.ParseInt(val, 10, 8)
	if err != nil {
		return -1, err
	}
	return int8(i64), nil
}

func (c *Cache) Set(ctx context.Context, userID string, roleID int8) error {
	key := helper.Key("role", "user", userID)
	return c.rdb.Set(ctx, key, strconv.FormatInt(int64(roleID), 10), c.ttl).Err()
}

func (c *Cache) Delete(ctx context.Context, userID string) error {
	return c.rdb.Del(ctx, helper.Key("role", "user", userID)).Err()
}
