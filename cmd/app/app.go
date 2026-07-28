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
	"github.com/motixo/goat-api/internal/usecase/authentication"
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
	newPostgres    func(context.Context, postgres.ClientConfig, pkg.Logger, service.PasswordHasher) (postgresResource, error)
	newRedis       func(context.Context, *config.Config, pkg.Logger) (redisResource, error)
	validateAssets func(context.Context, *redis.Client) error
	buildRuntime   func(*config.Config, pkg.Logger, *sqlx.DB, *redis.Client, service.PasswordHasher) (runtimeResources, error)
}

func InitializeApp(ctx context.Context, cfg *config.Config) (*Application, error) {
	return initializeApp(ctx, cfg, defaultBootstrapDependencies())
}

func initializeApp(
	ctx context.Context,
	cfg *config.Config,
	dependencies bootstrapDependencies,
) (*Application, error) {
	if ctx == nil {
		return nil, errors.New("application startup context is required")
	}
	if cfg == nil {
		return nil, errors.New("application configuration is required")
	}
	if err := startupContextError(ctx); err != nil {
		return nil, err
	}

	appLogger, err := dependencies.newLogger()
	if err != nil {
		return nil, fmt.Errorf("initialize logger: %w", err)
	}
	if err := startupContextError(ctx); err != nil {
		return nil, errors.Join(
			err,
			runCleanup("sync logger after startup cancellation", appLogger.sync),
		)
	}

	passwordHasher, err := authInfra.NewPasswordService(newPasswordHasherConfig(cfg))
	if err != nil {
		return nil, errors.Join(
			fmt.Errorf("initialize password hasher: %w", err),
			runCleanup("sync logger after password hasher initialization failure", appLogger.sync),
		)
	}

	database, err := dependencies.newPostgres(
		ctx,
		newPostgresClientConfig(cfg),
		appLogger.logger,
		passwordHasher,
	)
	if err != nil {
		return nil, errors.Join(
			fmt.Errorf("initialize postgres: %w", err),
			startupContextError(ctx),
			runCleanup("sync logger after PostgreSQL initialization failure", appLogger.sync),
		)
	}
	if err := startupContextError(ctx); err != nil {
		return nil, errors.Join(
			err,
			runCleanup("close PostgreSQL after startup cancellation", database.close),
			runCleanup("sync logger after startup cancellation", appLogger.sync),
		)
	}

	redisClient, err := dependencies.newRedis(ctx, cfg, appLogger.logger)
	if err != nil {
		return nil, errors.Join(
			fmt.Errorf("initialize Redis: %w", err),
			startupContextError(ctx),
			runCleanup("close PostgreSQL after Redis initialization failure", database.close),
			runCleanup("sync logger after Redis initialization failure", appLogger.sync),
		)
	}
	if err := startupContextError(ctx); err != nil {
		return nil, errors.Join(
			err,
			runCleanup("close Redis after startup cancellation", redisClient.close),
			runCleanup("close PostgreSQL after startup cancellation", database.close),
			runCleanup("sync logger after startup cancellation", appLogger.sync),
		)
	}

	validationCtx, cancelValidation := context.WithTimeout(
		ctx,
		runtimeAssetValidationTimeout,
	)
	if dependencies.validateAssets == nil {
		err = errors.New("runtime asset validator is required")
	} else {
		err = dependencies.validateAssets(validationCtx, redisClient.client)
	}
	err = errors.Join(err, startupContextError(validationCtx))
	cancelValidation()
	if err != nil {
		return nil, errors.Join(
			fmt.Errorf("validate runtime assets: %w", err),
			runCleanup("close Redis after runtime asset validation failure", redisClient.close),
			runCleanup("close PostgreSQL after runtime asset validation failure", database.close),
			runCleanup("sync logger after runtime asset validation failure", appLogger.sync),
		)
	}
	if err := startupContextError(ctx); err != nil {
		return nil, errors.Join(
			err,
			runCleanup("close Redis after startup cancellation", redisClient.close),
			runCleanup("close PostgreSQL after startup cancellation", database.close),
			runCleanup("sync logger after startup cancellation", appLogger.sync),
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
			startupContextError(ctx),
			runCleanup("close Redis after runtime initialization failure", redisClient.close),
			runCleanup("close PostgreSQL after runtime initialization failure", database.close),
			runCleanup("sync logger after runtime initialization failure", appLogger.sync),
		)
	}
	if err := startupContextError(ctx); err != nil {
		return nil, errors.Join(
			err,
			runCleanup("close Redis after startup cancellation", redisClient.close),
			runCleanup("close PostgreSQL after startup cancellation", database.close),
			runCleanup("sync logger after startup cancellation", appLogger.sync),
		)
	}

	return newApplication(applicationResources{
		address:         cfg.ServerAddress(),
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
			ctx context.Context,
			cfg postgres.ClientConfig,
			appLogger pkg.Logger,
			passwordHasher service.PasswordHasher,
		) (postgresResource, error) {
			db, err := postgres.NewDatabase(ctx, cfg, appLogger, passwordHasher)
			if err != nil {
				return postgresResource{}, err
			}
			return postgresResource{
				db:    db,
				close: db.Close,
			}, nil
		},
		newRedis: func(
			ctx context.Context,
			cfg *config.Config,
			appLogger pkg.Logger,
		) (redisResource, error) {
			client, err := redisStorage.NewClient(
				ctx,
				newRedisClientConfig(cfg),
				appLogger,
			)
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

func startupContextError(ctx context.Context) error {
	if ctx == nil || ctx.Err() == nil {
		return nil
	}
	contextErr := fmt.Errorf("application startup context: %w", ctx.Err())
	cause := context.Cause(ctx)
	if cause == nil || errors.Is(contextErr, cause) {
		return contextErr
	}
	return errors.Join(
		contextErr,
		fmt.Errorf("application startup cancellation cause: %w", cause),
	)
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
	authenticationUseCase := authentication.NewUsecase(authentication.Dependencies{
		UserRepository:      userRepository,
		SecurityStateReader: userRepository,
		SessionUseCase:      sessionUseCase,
		PasswordHasher:      passwordHasher,
		JWTService:          jwtManager,
		Logger:              appLogger,
		AccessTTL:           authentication.AccessTTL(cfg.JWTExpiration),
		RefreshTTL:          authentication.RefreshTTL(cfg.RefreshTokenExpiration),
		SessionTTL:          authentication.SessionTTL(cfg.SessionExpiration),
	})
	userUseCase := user.NewUsecase(user.Dependencies{
		UserRepository:               userRepository,
		DetailReader:                 userRepository,
		ListReader:                   userRepository,
		StatusReader:                 userRepository,
		UpdateWriter:                 userRepository,
		EmailWriter:                  userRepository,
		RoleWriter:                   userRepository,
		PasswordHasher:               passwordHasher,
		Logger:                       appLogger,
		SessionRepository:            sessionRepository,
		PasswordChangeCleanupMetrics: metricsService,
	})
	permissionUseCase := permission.NewUsecase(permissionRepository, appLogger)
	cleaner := cron.NewSessionCleaner(sessionRepository, appLogger)

	server, err := deliveryHTTP.NewServer(
		deliveryHTTP.GinMode(cfg.GinMode),
		deliveryHTTP.ServerDependencies{
			UserUseCase:           userUseCase,
			AuthenticationUseCase: authenticationUseCase,
			AuthorizationUseCase:  authorizationUseCase,
			PermissionUseCase:     permissionUseCase,
			SessionUseCase:        sessionUseCase,
			Logger:                appLogger,
			JWTService:            jwtManager,
			MetricsService:        metricsService,
			RateLimiter:           rateLimiter,
		},
		newRateLimitConfig(cfg),
	)
	if err != nil {
		return runtimeResources{}, fmt.Errorf("construct HTTP server: %w", err)
	}

	return runtimeResources{
		server:  server,
		cleaner: cleaner,
	}, nil
}

func newRedisClientConfig(cfg *config.Config) redisStorage.ClientConfig {
	return redisStorage.ClientConfig{
		Host:              cfg.RedisHost,
		Port:              cfg.RedisPort,
		Password:          cfg.RedisPassword,
		Database:          cfg.RedisDB,
		ConnectionTimeout: cfg.RedisConnectionTimeout,
	}
}

func newPasswordHasherConfig(cfg *config.Config) authInfra.PasswordHasherConfig {
	return authInfra.PasswordHasherConfig{
		Pepper:         cfg.PasswordPepper,
		MaxConcurrency: cfg.PasswordHashMaxConcurrency,
	}
}

func newPostgresClientConfig(cfg *config.Config) postgres.ClientConfig {
	return postgres.ClientConfig{
		Host:                  cfg.DBHost,
		Port:                  cfg.DBPort,
		User:                  cfg.DBUser,
		Password:              cfg.DBPassword,
		Database:              cfg.DBName,
		SSLMode:               postgres.SSLModeDisable,
		ConnectionTimeout:     cfg.DBConnectionTimeout,
		InitializationTimeout: cfg.DBInitializationTimeout,
		Seed:                  cfg.Seed,
		AdminEmail:            cfg.AdminEmail,
		AdminPassword:         cfg.AdminPassword,
	}
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
