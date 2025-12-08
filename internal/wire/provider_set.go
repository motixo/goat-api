package wire

import (
	"time"

	"github.com/google/wire"
	"github.com/jmoiron/sqlx"
	"github.com/redis/go-redis/v9"

	"github.com/motixo/goat-api/internal/config"
	"github.com/motixo/goat-api/internal/delivery/http"
	"github.com/motixo/goat-api/internal/delivery/http/handlers"
	"github.com/motixo/goat-api/internal/delivery/http/middleware"

	// Domain layer
	"github.com/motixo/goat-api/internal/domain/repository"
	"github.com/motixo/goat-api/internal/domain/service"
	"github.com/motixo/goat-api/internal/domain/usecase/auth"
	"github.com/motixo/goat-api/internal/domain/usecase/permission"
	"github.com/motixo/goat-api/internal/domain/usecase/session"
	"github.com/motixo/goat-api/internal/domain/usecase/user"

	// Infrastructure layer
	authInfra "github.com/motixo/goat-api/internal/infrastructure/auth"
	permissionCache "github.com/motixo/goat-api/internal/infrastructure/cache/permission"
	"github.com/motixo/goat-api/internal/infrastructure/database/postgres"
	postgresPermission "github.com/motixo/goat-api/internal/infrastructure/database/postgres/permission"
	postgresUser "github.com/motixo/goat-api/internal/infrastructure/database/postgres/user"
	"github.com/motixo/goat-api/internal/infrastructure/logger"
	redisSession "github.com/motixo/goat-api/internal/infrastructure/storage/redis/session"
)

// Infrastructure providers
var InfrastructureSet = wire.NewSet(
	config.Load,
	ProvideRedisClient,
	postgres.NewDatabase,
)

// Repository providers
var RepositorySet = wire.NewSet(
	postgresUser.NewRepository,
	postgresPermission.NewRepository,
	redisSession.NewRepository,
	NewPermissionRepository,
)

// Service providers
var ServiceSet = wire.NewSet(
	service.NewULIDGenerator,
	authInfra.NewPasswordService,
	NewJWTManager,
	NewZapLogger,
	wire.Bind(new(service.JWTService), new(*authInfra.JWTManager)),
	wire.Bind(new(logger.Logger), new(*logger.ZapLogger)),
)

// Configuration providers
var ConfigSet = wire.NewSet(
	ProvideAccessTTL,
	ProvideRefreshTTL,
	ProvideSessionTTL,
)

// UseCase providers - Wire will automatically use your constructor!
var UseCaseSet = wire.NewSet(
	auth.NewUsecase,    // Wire will match parameters by type
	session.NewUsecase, // You'll need to fix session constructor too
	user.NewUsecase,
	permission.NewUsecase,
)

// HTTP providers
var HTTPSet = wire.NewSet(
	handlers.NewAuthHandler,
	handlers.NewUserHandler,
	handlers.NewSessionHandler,
	middleware.NewAuthMiddleware,
	middleware.NewPermMiddleware,
	http.NewServer,
)

// ProviderSet bundles everything
var ProviderSet = wire.NewSet(
	InfrastructureSet,
	RepositorySet,
	ServiceSet,
	ConfigSet,
	UseCaseSet,
	HTTPSet,
)

// Infrastructure providers
func ProvideRedisClient(cfg *config.Config) *redis.Client {
	return redis.NewClient(&redis.Options{
		Addr:     cfg.RedisAddr,
		Password: cfg.RedisPassword,
		DB:       cfg.RedisDB,
	})
}

// Configuration providers with unique types
func ProvideAccessTTL(cfg *config.Config) auth.AccessTTL {
	return auth.AccessTTL(cfg.JWTExpiration)
}

func ProvideRefreshTTL(cfg *config.Config) auth.RefreshTTL {
	return auth.RefreshTTL(cfg.RefreshTokenExpiration)
}

func ProvideSessionTTL(cfg *config.Config) auth.SessionTTL {
	return auth.SessionTTL(cfg.SessionExpiration)
}

// Service providers
func NewJWTManager(cfg *config.Config) *authInfra.JWTManager {
	return authInfra.NewJWTManager(cfg.JWTSecret)
}

func NewZapLogger() (*logger.ZapLogger, error) {
	return logger.NewZapLogger()
}

// This function creates the complete cached repository
func NewPermissionRepository(
	db *sqlx.DB,
	redisClient *redis.Client,
	logger logger.Logger,
) repository.PermissionRepository {

	dbRepo := postgresPermission.NewRepository(db)
	cache := permissionCache.NewCache(redisClient, 24*time.Hour)

	return permissionCache.NewCachedRepository(dbRepo, cache, logger)
}
