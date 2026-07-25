package redis

import (
	"context"
	"errors"
	"fmt"

	"github.com/motixo/goat-api/internal/config"
	"github.com/motixo/goat-api/internal/pkg"
	"github.com/redis/go-redis/v9"
)

func NewClient(cfg *config.Config, logger pkg.Logger) (*redis.Client, error) {
	rdb := redis.NewClient(cfg.RedisOptions())

	if err := rdb.Ping(context.Background()).Err(); err != nil {
		logger.Error("failed to connect to Redis", "error", err)
		return nil, errors.Join(
			fmt.Errorf("failed to connect to redis: %w", err),
			closeFailedClient(rdb),
		)
	}

	logger.Info("Redis connected successfully", "addr", rdb.Options().Addr)
	return rdb, nil
}

func closeFailedClient(client *redis.Client) error {
	if err := client.Close(); err != nil {
		return fmt.Errorf("close Redis after initialization failure: %w", err)
	}
	return nil
}
