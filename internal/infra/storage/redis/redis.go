package redis

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/motixo/goat-api/internal/pkg"
	"github.com/redis/go-redis/v9"
)

// ClientConfig contains the values required to construct and validate a Redis client.
type ClientConfig struct {
	Host              string
	Port              uint16
	Password          string
	Database          int
	ConnectionTimeout time.Duration
}

// String formats the client configuration without exposing credentials.
func (c ClientConfig) String() string {
	return fmt.Sprintf(
		"{Host:%q Port:%d Password:<redacted> Database:%d ConnectionTimeout:%s}",
		c.Host,
		c.Port,
		c.Database,
		c.ConnectionTimeout,
	)
}

// GoString formats the client configuration without exposing credentials.
func (c ClientConfig) GoString() string {
	return c.String()
}

func (c ClientConfig) validate() error {
	if strings.TrimSpace(c.Host) == "" {
		return errors.New("redis host is required")
	}
	if c.Port == 0 {
		return errors.New("redis port must be positive")
	}
	if c.Database < 0 {
		return errors.New("redis database must not be negative")
	}
	if c.ConnectionTimeout <= 0 {
		return errors.New("redis connection timeout must be positive")
	}
	return nil
}

func newClientOptions(cfg ClientConfig) *redis.Options {
	return &redis.Options{
		Addr:                  fmt.Sprintf("%s:%d", cfg.Host, cfg.Port),
		Password:              cfg.Password,
		DB:                    cfg.Database,
		ContextTimeoutEnabled: true,
	}
}

type startupRedisClient interface {
	Ping(context.Context) *redis.StatusCmd
	Close() error
}

func NewClient(
	ctx context.Context,
	cfg ClientConfig,
	logger pkg.Logger,
) (*redis.Client, error) {
	if ctx == nil {
		return nil, errors.New("redis startup context is required")
	}
	if err := cfg.validate(); err != nil {
		return nil, err
	}

	rdb := redis.NewClient(newClientOptions(cfg))

	if err := initializeRedisClient(ctx, cfg.ConnectionTimeout, rdb); err != nil {
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
