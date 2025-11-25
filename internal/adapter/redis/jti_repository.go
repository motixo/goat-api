package redis

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"
)

type JTIRepository struct {
	client *redis.Client
}

func NewJTIRepository(client *redis.Client) *JTIRepository {
	return &JTIRepository{client: client}
}

func (r *JTIRepository) SaveJTI(ctx context.Context, userID string, jti string, ttlSeconds int) error {
	return r.client.Set(ctx, jti, userID, time.Duration(ttlSeconds)*time.Second).Err()
}

func (r *JTIRepository) Exists(ctx context.Context, jti string) (bool, error) {
	val, err := r.client.Exists(ctx, jti).Result()
	return val == 1, err
}

func (r *JTIRepository) DeleteJTI(ctx context.Context, jti string) error {
	return r.client.Del(ctx, jti).Err()
}
