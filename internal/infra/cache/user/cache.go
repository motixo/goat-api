package usercache

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/motixo/goat-api/internal/domain/entity"
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

func (c *Cache) Get(ctx context.Context, userID string) (*userAuthorization, error) {
	if err := validateUserCacheID(userID); err != nil {
		return nil, err
	}

	key := pkg.RedisKey("user", "id", userID)
	val, err := c.rdb.Get(ctx, key).Result()
	if errors.Is(err, redis.Nil) {
		return nil, nil // cache miss
	}
	if err != nil {
		return nil, fmt.Errorf("read user authorization cache for %q: %w", userID, err)
	}

	var record *userCacheRecord
	if err := json.Unmarshal([]byte(val), &record); err != nil {
		return nil, fmt.Errorf("decode user authorization cache for %q: %w", userID, err)
	}
	if record == nil {
		return nil, fmt.Errorf("decode user authorization cache for %q: payload must be a JSON object", userID)
	}
	authorization, err := record.toAuthorization(userID)
	if err != nil {
		return nil, fmt.Errorf("decode user authorization cache for %q: %w", userID, err)
	}
	return authorization, nil
}

func (c *Cache) Set(ctx context.Context, userID string, user *entity.User) error {
	record, err := userCacheRecordFromDomain(user, userID)
	if err != nil {
		return fmt.Errorf("encode user authorization cache for %q: %w", userID, err)
	}

	jsonData, err := json.Marshal(record)
	if err != nil {
		return fmt.Errorf("encode user authorization cache for %q: %w", userID, err)
	}

	key := pkg.RedisKey("user", "id", userID)
	if err := c.rdb.Set(ctx, key, jsonData, c.ttl).Err(); err != nil {
		return fmt.Errorf("write user authorization cache for %q: %w", userID, err)
	}
	return nil
}

func (c *Cache) Delete(ctx context.Context, userID string) error {
	if err := validateUserCacheID(userID); err != nil {
		return err
	}
	if err := c.rdb.Del(ctx, pkg.RedisKey("user", "id", userID)).Err(); err != nil {
		return fmt.Errorf("delete user authorization cache for %q: %w", userID, err)
	}
	return nil
}
