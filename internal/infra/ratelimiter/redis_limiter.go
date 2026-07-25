package ratelimiter

import (
	"context"
	"fmt"
	"time"

	"github.com/motixo/goat-api/internal/domain/service"
	redisClinet "github.com/motixo/goat-api/internal/infra/storage/redis"
	"github.com/motixo/goat-api/internal/pkg"
	"github.com/redis/go-redis/v9"
)

type RedisRateLimiter struct {
	redis *redis.Client
}

func NewRedisRateLimiter(redis *redis.Client) service.RateLimiter {
	return &RedisRateLimiter{
		redis: redis,
	}
}

func (r *RedisRateLimiter) Allow(
	ctx context.Context,
	actorType string,
	actorID string,
	resource string,
	limit int,
	windowDuration time.Duration,
) (bool, time.Duration, int64, error) {
	nowMicro := time.Now().UTC().UnixMicro()
	windowMicro := windowDuration.Microseconds()
	windowSeconds := int64(windowDuration.Seconds())

	key := pkg.RedisKey("rl", actorType, fmt.Sprintf("%s:%s", actorID, resource))

	// If the window is less than 1 second, ensure EXPIRE is at least 1s
	if windowSeconds == 0 {
		windowSeconds = 1
	}

	member := pkg.ULIDGenerator()
	script, err := redisClinet.GetScript(redisClinet.ScriptRateLimit)
	if err != nil {
		return false, 0, 0, fmt.Errorf("resolve rate-limit script: %w", err)
	}
	result, err := script.Run(ctx, r.redis, []string{key},
		limit,
		windowMicro,
		nowMicro,
		member,
		windowSeconds,
	).Result()

	if err != nil {
		return false, 0, 0, err
	}

	res := result.([]interface{})
	allowed := res[0].(int64) == 1
	retryAfterMicro := res[1].(int64)
	currentCount := res[2].(int64)

	return allowed, time.Duration(retryAfterMicro) * time.Microsecond, currentCount, nil
}
