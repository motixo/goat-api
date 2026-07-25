package main

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/motixo/goat-api/internal/config"
	"github.com/motixo/goat-api/internal/cron"
	deliveryHTTP "github.com/motixo/goat-api/internal/delivery/http"
	"github.com/motixo/goat-api/internal/delivery/http/middleware"
	"github.com/motixo/goat-api/internal/delivery/http/response"
	"github.com/motixo/goat-api/internal/domain/service"
	authInfra "github.com/motixo/goat-api/internal/infra/auth"
	"github.com/motixo/goat-api/internal/infra/database/postgres"
	postgresPermission "github.com/motixo/goat-api/internal/infra/database/postgres/permission"
	postgresUser "github.com/motixo/goat-api/internal/infra/database/postgres/user"
	"github.com/motixo/goat-api/internal/infra/logger"
	"github.com/motixo/goat-api/internal/infra/metrics"
	"github.com/motixo/goat-api/internal/infra/ratelimiter"
	redisStorage "github.com/motixo/goat-api/internal/infra/storage/redis"
	redisSession "github.com/motixo/goat-api/internal/infra/storage/redis/session"
	"github.com/motixo/goat-api/internal/pkg"
	"github.com/motixo/goat-api/internal/usecase/auth"
	"github.com/motixo/goat-api/internal/usecase/authorization"
	"github.com/motixo/goat-api/internal/usecase/permission"
	"github.com/motixo/goat-api/internal/usecase/session"
	"github.com/motixo/goat-api/internal/usecase/user"
	"github.com/redis/go-redis/v9"
)

const (
	defaultShutdownTimeout        = 15 * time.Second
	runtimeAssetValidationTimeout = 5 * time.Second
)

type loggerResource struct {
	logger pkg.Logger
	sync   func() error
}

type postgresResource struct {
	db    *sqlx.DB
	close func() error
}

type redisResource struct {
	client *redis.Client
	close  func() error
}

type runtimeResources struct {
	server  lifecycleServer
	cleaner lifecycleWorker
}

type bootstrapDependencies struct {
	newLogger      func() (loggerResource, error)
	newPostgres    func(*config.Config, pkg.Logger, service.PasswordHasher) (postgresResource, error)
	newRedis       func(*config.Config, pkg.Logger) (redisResource, error)
	validateAssets func(context.Context, *redis.Client) error
	buildRuntime   func(*config.Config, pkg.Logger, *sqlx.DB, *redis.Client, service.PasswordHasher) (runtimeResources, error)
}

func InitializeApp(cfg *config.Config) (*Application, error) {
	return initializeApp(cfg, defaultBootstrapDependencies())
}

func initializeApp(cfg *config.Config, dependencies bootstrapDependencies) (*Application, error) {
	appLogger, err := dependencies.newLogger()
	if err != nil {
		return nil, fmt.Errorf("initialize logger: %w", err)
	}

	passwordHasher := authInfra.NewPasswordService(cfg)

	database, err := dependencies.newPostgres(cfg, appLogger.logger, passwordHasher)
	if err != nil {
		return nil, errors.Join(
			fmt.Errorf("initialize postgres: %w", err),
			runCleanup("sync logger after PostgreSQL initialization failure", appLogger.sync),
		)
	}

	redisClient, err := dependencies.newRedis(cfg, appLogger.logger)
	if err != nil {
		return nil, errors.Join(
			fmt.Errorf("initialize Redis: %w", err),
			runCleanup("close PostgreSQL after Redis initialization failure", database.close),
			runCleanup("sync logger after Redis initialization failure", appLogger.sync),
		)
	}

	validationCtx, cancelValidation := context.WithTimeout(
		context.Background(),
		runtimeAssetValidationTimeout,
	)
	if dependencies.validateAssets == nil {
		err = errors.New("runtime asset validator is required")
	} else {
		err = dependencies.validateAssets(validationCtx, redisClient.client)
	}
	cancelValidation()
	if err != nil {
		return nil, errors.Join(
			fmt.Errorf("validate runtime assets: %w", err),
			runCleanup("close Redis after runtime asset validation failure", redisClient.close),
			runCleanup("close PostgreSQL after runtime asset validation failure", database.close),
			runCleanup("sync logger after runtime asset validation failure", appLogger.sync),
		)
	}

	runtime, err := dependencies.buildRuntime(
		cfg,
		appLogger.logger,
		database.db,
		redisClient.client,
		passwordHasher,
	)
	if err != nil {
		return nil, errors.Join(
			fmt.Errorf("initialize application runtime: %w", err),
			runCleanup("close Redis after runtime initialization failure", redisClient.close),
			runCleanup("close PostgreSQL after runtime initialization failure", database.close),
			runCleanup("sync logger after runtime initialization failure", appLogger.sync),
		)
	}

	return newApplication(applicationResources{
		address:         cfg.ServerPort,
		shutdownTimeout: defaultShutdownTimeout,
		server:          runtime.server,
		cleaner:         runtime.cleaner,
		closeRedis:      redisClient.close,
		closePostgres:   database.close,
		syncLogger:      appLogger.sync,
	}), nil
}

func defaultBootstrapDependencies() bootstrapDependencies {
	return bootstrapDependencies{
		newLogger: func() (loggerResource, error) {
			appLogger, err := logger.NewZapLogger()
			if err != nil {
				return loggerResource{}, err
			}
			return loggerResource{
				logger: appLogger,
				sync:   appLogger.Sync,
			}, nil
		},
		newPostgres: func(
			cfg *config.Config,
			appLogger pkg.Logger,
			passwordHasher service.PasswordHasher,
		) (postgresResource, error) {
			db, err := postgres.NewDatabase(cfg, appLogger, passwordHasher)
			if err != nil {
				return postgresResource{}, err
			}
			return postgresResource{
				db:    db,
				close: db.Close,
			}, nil
		},
		newRedis: func(cfg *config.Config, appLogger pkg.Logger) (redisResource, error) {
			client, err := redisStorage.NewClient(cfg, appLogger)
			if err != nil {
				return redisResource{}, err
			}
			return redisResource{
				client: client,
				close:  client.Close,
			}, nil
		},
		validateAssets: validateRuntimeAssets,
		buildRuntime:   buildRuntime,
	}
}

func validateRuntimeAssets(ctx context.Context, redisClient *redis.Client) error {
	var validationErrors []error
	if err := response.ValidateRuntimeAssets(); err != nil {
		validationErrors = append(
			validationErrors,
			fmt.Errorf("validate HTTP problem localization assets: %w", err),
		)
	}
	if err := redisStorage.ValidateRuntimeScripts(ctx, redisClient); err != nil {
		validationErrors = append(
			validationErrors,
			fmt.Errorf("validate Redis scripts: %w", err),
		)
	}
	return errors.Join(validationErrors...)
}

func buildRuntime(
	cfg *config.Config,
	appLogger pkg.Logger,
	db *sqlx.DB,
	redisClient *redis.Client,
	passwordHasher service.PasswordHasher,
) (runtimeResources, error) {
	userRepository := postgresUser.NewRepository(db)
	permissionRepository := postgresPermission.NewRepository(db)
	sessionRepository := redisSession.NewRepository(redisClient, appLogger)

	jwtManager := authInfra.NewJWTManager(cfg.JWTSecret)
	metricsService := metrics.NewPrometheusMetrics()
	rateLimiter := ratelimiter.NewRedisRateLimiter(redisClient)

	sessionUseCase := session.NewUsecase(sessionRepository, appLogger)
	authorizationUseCase := authorization.NewUsecase(
		userRepository,
		userRepository,
	)
	authUseCase := auth.NewUsecase(
		userRepository,
		userRepository,
		sessionUseCase,
		passwordHasher,
		jwtManager,
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
		metricsService,
	)
	permissionUseCase := permission.NewUsecase(permissionRepository, appLogger)
	cleaner := cron.NewSessionCleaner(sessionRepository, appLogger)

	server := deliveryHTTP.NewServer(
		userUseCase,
		authUseCase,
		authorizationUseCase,
		permissionUseCase,
		sessionUseCase,
		appLogger,
		jwtManager,
		metricsService,
		rateLimiter,
		newRateLimitConfig(cfg),
	)

	return runtimeResources{
		server:  server,
		cleaner: cleaner,
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
