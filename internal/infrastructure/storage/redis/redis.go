package redis

import (
	"context"
	"fmt"

	"github.com/motixo/goat-api/internal/config"
	"github.com/motixo/goat-api/internal/infrastructure/logger"
	"github.com/redis/go-redis/v9"
)

type RedisClientInterface interface {
	Ping(ctx context.Context) error
	Client() *redis.Client
}

type RedisClient struct {
	client *redis.Client
}

func (r *RedisClient) Client() *redis.Client {
	return r.client
}

func (r *RedisClient) Ping(ctx context.Context) error {
	return r.client.Ping(ctx).Err()
}

func NewClient(cfg *config.Config, logger logger.Logger) (RedisClientInterface, error) {
	rdb := redis.NewClient(&redis.Options{
		Addr:     cfg.RedisAddr,
		Password: cfg.RedisPassword,
		DB:       cfg.RedisDB,
	})

	if err := rdb.Ping(context.Background()).Err(); err != nil {
		logger.Error("failed to connect to Redis", "error", err)
		return nil, fmt.Errorf("failed to connect to redis: %w", err)
	}

	logger.Info("Redis connected successfully", "addr", cfg.RedisAddr)
	return &RedisClient{client: rdb}, nil
}
