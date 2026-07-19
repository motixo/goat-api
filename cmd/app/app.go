package main

import (
	"context"
	"fmt"
	"reflect"

	"github.com/motixo/goat-api/internal/config"
	"github.com/motixo/goat-api/internal/cron"
	deliveryHTTP "github.com/motixo/goat-api/internal/delivery/http"
	"github.com/motixo/goat-api/internal/delivery/http/middleware"
	domainEvent "github.com/motixo/goat-api/internal/domain/event"
	"github.com/motixo/goat-api/internal/domain/service"
	authInfra "github.com/motixo/goat-api/internal/infra/auth"
	permcache "github.com/motixo/goat-api/internal/infra/cache/permission"
	usercache "github.com/motixo/goat-api/internal/infra/cache/user"
	"github.com/motixo/goat-api/internal/infra/database/postgres"
	postgresPermission "github.com/motixo/goat-api/internal/infra/database/postgres/permission"
	postgresUser "github.com/motixo/goat-api/internal/infra/database/postgres/user"
	"github.com/motixo/goat-api/internal/infra/event"
	"github.com/motixo/goat-api/internal/infra/logger"
	"github.com/motixo/goat-api/internal/infra/metrics"
	"github.com/motixo/goat-api/internal/infra/ratelimiter"
	redisStorage "github.com/motixo/goat-api/internal/infra/storage/redis"
	redisSession "github.com/motixo/goat-api/internal/infra/storage/redis/session"
	"github.com/motixo/goat-api/internal/pkg"
	"github.com/motixo/goat-api/internal/usecase/auth"
	"github.com/motixo/goat-api/internal/usecase/permission"
	"github.com/motixo/goat-api/internal/usecase/session"
	"github.com/motixo/goat-api/internal/usecase/user"
)

type AppContext struct {
	Server   *deliveryHTTP.Server
	EventBus *event.InMemoryPublisher
	Cleaner  *cron.SessionCleaner
}

func InitializeApp(cfg *config.Config) (*AppContext, error) {
	appLogger, err := logger.NewZapLogger()
	if err != nil {
		return nil, fmt.Errorf("initialize logger: %w", err)
	}

	passwordHasher := authInfra.NewPasswordService(cfg)

	db, err := postgres.NewDatabase(cfg, appLogger, passwordHasher)
	if err != nil {
		return nil, fmt.Errorf("initialize postgres: %w", err)
	}

	redisClient, err := redisStorage.NewClient(cfg, appLogger)
	if err != nil {
		return nil, fmt.Errorf("initialize redis: %w", err)
	}

	userRepository := postgresUser.NewRepository(db)
	permissionRepository := postgresPermission.NewRepository(db)
	sessionRepository := redisSession.NewRepository(redisClient, appLogger)

	userCache := usercache.NewCache(redisClient)
	permissionCache := permcache.NewCache(redisClient)
	userCacheService := usercache.NewCachedRepository(userRepository, userCache, appLogger)
	permissionCacheService := permcache.NewCachedRepository(permissionRepository, permissionCache, appLogger)

	eventBus := newConfiguredEventBus(appLogger, userCacheService, permissionCacheService)
	jwtManager := authInfra.NewJWTManager(cfg.JWTSecret)
	metricsService := metrics.NewPrometheusMetrics()
	rateLimiter := ratelimiter.NewRedisRateLimiter(redisClient)

	sessionUseCase := session.NewUsecase(sessionRepository, appLogger)
	authUseCase := auth.NewUsecase(
		userRepository,
		sessionUseCase,
		passwordHasher,
		jwtManager,
		userCacheService,
		appLogger,
		auth.AccessTTL(cfg.JWTExpiration),
		auth.RefreshTTL(cfg.RefreshTokenExpiration),
		auth.SessionTTL(cfg.SessionExpiration),
	)
	userUseCase := user.NewUsecase(
		userRepository,
		passwordHasher,
		appLogger,
		sessionRepository,
		userCacheService,
		eventBus,
	)
	permissionUseCase := permission.NewUsecase(permissionRepository, eventBus, appLogger)

	server := deliveryHTTP.NewServer(
		userUseCase,
		authUseCase,
		permissionUseCase,
		permissionCacheService,
		sessionUseCase,
		userCacheService,
		appLogger,
		jwtManager,
		metricsService,
		rateLimiter,
		newRateLimitConfig(cfg),
	)

	return &AppContext{
		Server:   server,
		EventBus: eventBus,
		Cleaner:  cron.NewSessionCleaner(sessionRepository, appLogger),
	}, nil
}

func newRateLimitConfig(cfg *config.Config) middleware.RateLimitConfig {
	return middleware.RateLimitConfig{
		Auth: middleware.RateLimit{
			Limit:  cfg.RateLimitAuthLimit,
			Window: cfg.RateLimitAuthWindow,
		},
		Public: middleware.RateLimit{
			Limit:  cfg.RateLimitPublicLimit,
			Window: cfg.RateLimitPublicWindow,
		},
		Private: middleware.RateLimit{
			Limit:  cfg.RateLimitPrivateLimit,
			Window: cfg.RateLimitPrivateWindow,
		},
	}
}

func newConfiguredEventBus(
	appLogger pkg.Logger,
	userCacheService service.UserCacheService,
	permissionCacheService service.PermCacheService,
) *event.InMemoryPublisher {
	bus := event.NewInMemoryPublisher(appLogger)

	bus.RegisterHandler(
		reflect.TypeOf(domainEvent.UserUpdatedEvent{}),
		func(ctx context.Context, eventData any) error {
			event, ok := eventData.(domainEvent.UserUpdatedEvent)
			if !ok {
				return nil
			}
			return userCacheService.ClearCache(ctx, event.UserID)
		},
	)

	bus.RegisterHandler(
		reflect.TypeOf(domainEvent.PermissionUpdatedEvent{}),
		func(ctx context.Context, eventData any) error {
			event, ok := eventData.(domainEvent.PermissionUpdatedEvent)
			if !ok {
				return nil
			}
			return permissionCacheService.ClearCache(ctx, event.Role)
		},
	)

	return bus
}
