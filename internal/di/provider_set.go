package di

import (
	"context"
	"reflect"
	"time"

	"github.com/google/wire"
	"github.com/redis/go-redis/v9"

	"github.com/motixo/goat-api/internal/config"
	"github.com/motixo/goat-api/internal/delivery/http"
	"github.com/motixo/goat-api/internal/delivery/http/handlers"
	"github.com/motixo/goat-api/internal/delivery/http/middleware"

	// Domain layer
	domainEvent "github.com/motixo/goat-api/internal/domain/event"
	"github.com/motixo/goat-api/internal/domain/service"
	"github.com/motixo/goat-api/internal/domain/usecase/auth"
	"github.com/motixo/goat-api/internal/domain/usecase/permission"
	"github.com/motixo/goat-api/internal/domain/usecase/session"
	"github.com/motixo/goat-api/internal/domain/usecase/user"

	// infra layer
	authInfra "github.com/motixo/goat-api/internal/infra/auth"
	permcache "github.com/motixo/goat-api/internal/infra/cache/permission"
	usercache "github.com/motixo/goat-api/internal/infra/cache/user"
	"github.com/motixo/goat-api/internal/infra/database/postgres"
	postgresPermission "github.com/motixo/goat-api/internal/infra/database/postgres/permission"
	postgresUser "github.com/motixo/goat-api/internal/infra/database/postgres/user"
	"github.com/motixo/goat-api/internal/infra/event"
	"github.com/motixo/goat-api/internal/infra/logger"
	"github.com/motixo/goat-api/internal/infra/metrics"
	redisSession "github.com/motixo/goat-api/internal/infra/storage/redis/session"
)

// infra providers
var infraSet = wire.NewSet(
	config.Load,
	ProvideRedisClient,
	NewZapLogger,
	wire.Bind(new(service.Logger), new(*logger.ZapLogger)),
	authInfra.NewPasswordService,
	postgres.NewDatabase,
	NewUserCache,
	NewPermissionCache,
	usercache.NewCachedRepository,
	permcache.NewCachedRepository,

	ProvideConfiguredEventBus,
	wire.Bind(new(domainEvent.Publisher), new(*event.InMemoryPublisher)),
)

// Repository providers
var RepositorySet = wire.NewSet(
	postgresUser.NewRepository,
	postgresPermission.NewRepository,
	redisSession.NewRepository,
)

// Service providers
var ServiceSet = wire.NewSet(
	service.NewULIDGenerator,
	NewJWTManager,
	wire.Bind(new(service.JWTService), new(*authInfra.JWTManager)),
	metrics.NewPrometheusMetrics,
	wire.Bind(new(service.MetricsService), new(*metrics.PrometheusMetrics)),
)

// Configuration providers
var ConfigSet = wire.NewSet(
	ProvideAccessTTL,
	ProvideRefreshTTL,
	ProvideSessionTTL,
)

// UseCase provider
var UseCaseSet = wire.NewSet(
	auth.NewUsecase,
	session.NewUsecase,
	user.NewUsecase,
	permission.NewUsecase,
)

// HTTP providers
var HTTPSet = wire.NewSet(
	handlers.NewAuthHandler,
	handlers.NewUserHandler,
	handlers.NewSessionHandler,
	handlers.NewPermissionHandler,
	middleware.NewAuthMiddleware,
	middleware.NewPermMiddleware,
	middleware.NewMetricsMiddleware,
	http.NewServer,
)

// ProviderSet bundles everything
var ProviderSet = wire.NewSet(
	infraSet,
	RepositorySet,
	ServiceSet,
	ConfigSet,
	UseCaseSet,
	HTTPSet,
)

// infra providers
func ProvideRedisClient(cfg *config.Config) *redis.Client {
	return redis.NewClient(&redis.Options{
		Addr:     cfg.RedisAddr,
		Password: cfg.RedisPassword,
		DB:       cfg.RedisDB,
	})
}

func ProvideAccessTTL(cfg *config.Config) auth.AccessTTL {
	return auth.AccessTTL(cfg.JWTExpiration)
}

func ProvideRefreshTTL(cfg *config.Config) auth.RefreshTTL {
	return auth.RefreshTTL(cfg.RefreshTokenExpiration)
}

func ProvideSessionTTL(cfg *config.Config) auth.SessionTTL {
	return auth.SessionTTL(cfg.SessionExpiration)
}

func NewJWTManager(cfg *config.Config) *authInfra.JWTManager {
	return authInfra.NewJWTManager(cfg.JWTSecret)
}

func NewZapLogger() (*logger.ZapLogger, error) {
	return logger.NewZapLogger()
}

func NewUserCache(redisClient *redis.Client) *usercache.Cache {
	return usercache.NewCache(redisClient, 24*time.Hour)
}

func NewPermissionCache(redisClient *redis.Client) *permcache.Cache {
	return permcache.NewCache(redisClient, 24*time.Hour)
}

func ProvideConfiguredEventBus(
	logger service.Logger,
	userCacheRepo service.UserCacheService,
	permCacheRepo service.PermCacheService,
) (*event.InMemoryPublisher, error) {
	bus := event.NewInMemoryPublisher(logger)

	// Register UserUpdatedEvent handler
	bus.RegisterHandler(
		reflect.TypeOf(domainEvent.UserUpdatedEvent{}),
		func(ctx context.Context, e any) error {
			evt, ok := e.(domainEvent.UserUpdatedEvent)
			if !ok {
				return nil // or log
			}
			return userCacheRepo.ClearCache(ctx, evt.UserID)
		},
	)

	// Register PermissionUpdatedEvent handler
	bus.RegisterHandler(
		reflect.TypeOf(domainEvent.PermissionUpdatedEvent{}),
		func(ctx context.Context, e any) error {
			evt, ok := e.(domainEvent.PermissionUpdatedEvent)
			if !ok {
				return nil
			}
			return permCacheRepo.ClearCache(ctx, evt.Role)
		},
	)

	return bus, nil
}
