package wire

import (
	"time"

	"github.com/google/wire"
	"github.com/motixo/goth-api/internal/adapter/log"
	adapterPerm "github.com/motixo/goth-api/internal/adapter/permission"
	"github.com/motixo/goth-api/internal/adapter/postgres"
	postgresPerm "github.com/motixo/goth-api/internal/adapter/postgres/permission"
	postgresUser "github.com/motixo/goth-api/internal/adapter/postgres/user"
	redisPerm "github.com/motixo/goth-api/internal/adapter/redis/permission"
	redisSession "github.com/motixo/goth-api/internal/adapter/redis/session"
	"github.com/motixo/goth-api/internal/config"
	"github.com/motixo/goth-api/internal/delivery/http"
	"github.com/motixo/goth-api/internal/delivery/http/handlers"
	"github.com/motixo/goth-api/internal/delivery/http/middleware"
	"github.com/motixo/goth-api/internal/domain/service"
	"github.com/motixo/goth-api/internal/domain/usecase/auth"
	"github.com/motixo/goth-api/internal/domain/usecase/permission"
	"github.com/motixo/goth-api/internal/domain/usecase/session"
	"github.com/motixo/goth-api/internal/domain/usecase/user"
	"github.com/redis/go-redis/v9"
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
	redisSession.NewRepository,
	postgresPerm.NewRepository,
	ProvidePermissionCache,
	ProvideCachedPermissionRepository,
)

// Service providers
var ServiceSet = wire.NewSet(
	service.NewULIDGenerator,
	service.NewPasswordService,
)

// Usecase providers
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
	middleware.NewAuthMiddleware,
	middleware.NewPermMiddleware,
	http.NewServer,
)

var LoggerSet = wire.NewSet(
	log.NewZapLogger,
	wire.Bind(new(service.Logger), new(*log.ZapLogger)),
)

// ProviderSet bundles everything
var ProviderSet = wire.NewSet(
	InfrastructureSet,
	RepositorySet,
	ServiceSet,
	UseCaseSet,
	HTTPSet,
	LoggerSet,
)

func ProvideRedisClient(cfg *config.Config) *redis.Client {
	return redis.NewClient(&redis.Options{
		Addr:     cfg.RedisAddr,
		Password: cfg.RedisPassword,
		DB:       cfg.RedisDB,
	})
}

func ProvidePermissionCache(rdb *redis.Client) *redisPerm.Cache {
	return redisPerm.NewCache(rdb, 5*time.Minute)
}

func ProvideCachedPermissionRepository(
	dbRepo *postgresPerm.Repository,
	cache *redisPerm.Cache,
	logger service.Logger,
) permission.Repository {
	return adapterPerm.NewCachedRepository(dbRepo, cache, logger)
}
