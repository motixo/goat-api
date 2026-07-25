package redis

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/motixo/goat-api/internal/config"
	"github.com/motixo/goat-api/internal/pkg"
	"github.com/redis/go-redis/v9"
)

type startupRedisClient interface {
	Ping(context.Context) *redis.StatusCmd
	Close() error
}

func NewClient(
	ctx context.Context,
	cfg *config.Config,
	logger pkg.Logger,
) (*redis.Client, error) {
	if ctx == nil {
		return nil, errors.New("redis startup context is required")
	}
	if cfg == nil {
		return nil, errors.New("redis configuration is required")
	}
	if cfg.RedisConnectionTimeout <= 0 {
		return nil, errors.New("redis connection timeout must be positive")
	}

	rdb := redis.NewClient(cfg.RedisOptions())

	if err := initializeRedisClient(ctx, cfg.RedisConnectionTimeout, rdb); err != nil {
		logger.Error("failed to connect to Redis", "error", err)
		return nil, err
	}

	logger.Info("Redis connected successfully", "addr", rdb.Options().Addr)
	return rdb, nil
}

func initializeRedisClient(
	ctx context.Context,
	timeout time.Duration,
	client startupRedisClient,
) error {
	if ctx == nil {
		return errors.New("redis startup context is required")
	}
	if timeout <= 0 {
		return errors.New("redis connection timeout must be positive")
	}
	if client == nil {
		return errors.New("redis client is required")
	}

	validationCtx, cancelValidation := context.WithTimeout(ctx, timeout)
	err := client.Ping(validationCtx).Err()
	if err != nil {
		err = redisStartupOperationError("validate Redis connection", validationCtx, err)
	}
	cancelValidation()
	if err == nil {
		return nil
	}
	return errors.Join(err, closeFailedClient(client))
}

func redisStartupOperationError(operation string, ctx context.Context, err error) error {
	operationErr := fmt.Errorf("%s: %w", operation, err)
	cause := context.Cause(ctx)
	if cause == nil || errors.Is(operationErr, cause) {
		return operationErr
	}
	return errors.Join(
		operationErr,
		fmt.Errorf("%s context: %w", operation, cause),
	)
}

func closeFailedClient(client startupRedisClient) error {
	if err := client.Close(); err != nil {
		return fmt.Errorf("close Redis after initialization failure: %w", err)
	}
	return nil
}
