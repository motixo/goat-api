package main

import (
	"context"
	"errors"
	"fmt"

	"github.com/jmoiron/sqlx"
	deliveryHTTP "github.com/motixo/goat-api/internal/delivery/http"
	"github.com/redis/go-redis/v9"
)

type readinessProbe func(context.Context) error

// runtimeReadinessChecker composes the concrete runtime dependencies at the
// application boundary while satisfying the HTTP delivery-owned port.
type runtimeReadinessChecker struct {
	postgres readinessProbe
	redis    readinessProbe
}

func newRuntimeReadinessChecker(
	database *sqlx.DB,
	redisClient *redis.Client,
) deliveryHTTP.ReadinessChecker {
	return runtimeReadinessChecker{
		postgres: database.PingContext,
		redis: func(ctx context.Context) error {
			return redisClient.Ping(ctx).Err()
		},
	}
}

func (c runtimeReadinessChecker) CheckReadiness(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("runtime readiness canceled before dependency checks: %w", err)
	}
	if c.postgres == nil || c.redis == nil {
		return errors.New("runtime readiness dependency probes are incomplete")
	}
	if err := c.postgres(ctx); err != nil {
		return fmt.Errorf("check PostgreSQL readiness: %w", err)
	}
	if err := c.redis(ctx); err != nil {
		return fmt.Errorf("check Redis readiness: %w", err)
	}
	return nil
}
