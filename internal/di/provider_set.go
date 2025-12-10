package di

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

	// infra layer
	authInfra "github.com/motixo/goat-api/internal/infra/auth"
	permissionCache "github.com/motixo/goat-api/internal/infra/cache/permission"
	roleCache "github.com/motixo/goat-api/internal/infra/cache/role"
	"github.com/motixo/goat-api/internal/infra/database/postgres"
	postgresPermission "github.com/motixo/goat-api/internal/infra/database/postgres/permission"
	postgresUser "github.com/motixo/goat-api/internal/infra/database/postgres/user"
	"github.com/motixo/goat-api/internal/infra/logger"
	redisSession "github.com/motixo/goat-api/internal/infra/storage/redis/session"
)

// infra providers
var infraSet = wire.NewSet(
	config.Load,
	ProvideRedisClient,
	NewZapLogger,
	wire.Bind(new(logger.Logger), new(*logger.ZapLogger)),
	authInfra.NewPasswordService,
	postgres.NewDatabase,
)

// Repository providers
var RepositorySet = wire.NewSet(
	postgresUser.NewRepository,
	postgresPermission.NewRepository,
	redisSession.NewRepository,
	NewPermissionRepository,
	NewRoleRepository,
)

// Service providers
var ServiceSet = wire.NewSet(
	service.NewULIDGenerator,
	NewJWTManager,
	wire.Bind(new(service.JWTService), new(*authInfra.JWTManager)),
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

func NewRoleRepository(
	db *sqlx.DB,
	redisClient *redis.Client,
	logger logger.Logger,
) repository.RoleRepository {

	dbRepo := postgresUser.NewRepository(db)
	cache := roleCache.NewCache(redisClient, 24*time.Hour)

	return roleCache.NewCachedRepository(dbRepo, cache, logger)
}
