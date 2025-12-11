package usercache

import (
	"context"
	"encoding/json"
	"time"

	"github.com/motixo/goat-api/internal/infra/helper"
	"github.com/redis/go-redis/v9"
)

type Cache struct {
	rdb *redis.Client
	ttl time.Duration
}

func NewCache(rdb *redis.Client, ttl time.Duration) *Cache {
	return &Cache{rdb: rdb, ttl: ttl}
}

func (c *Cache) Get(ctx context.Context, userID string) (*UserCache, error) {
	key := helper.Key("user", "id", userID)
	val, err := c.rdb.Get(ctx, key).Result()
	if err == redis.Nil {
		return nil, nil // cache miss
	}
	if err != nil {
		return nil, err
	}

	var data UserCache
	if err := json.Unmarshal([]byte(val), &data); err != nil {
		return nil, err
	}

	return &data, nil
}

func (c *Cache) Set(ctx context.Context, userID string, role, status int8) error {
	key := helper.Key("user", "id", userID)
	data := UserCache{
		Role:   role,
		Status: status,
	}

	jsonData, err := json.Marshal(data)
	if err != nil {
		return err
	}

	return c.rdb.Set(ctx, key, jsonData, c.ttl).Err()
}

func (c *Cache) Delete(ctx context.Context, userID string) error {
	return c.rdb.Del(ctx, helper.Key("user", "id", userID)).Err()
}
