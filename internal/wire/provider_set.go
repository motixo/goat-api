package wire

import (
	"github.com/google/wire"
	"github.com/mot0x0/goth-api/internal/adapter/postgres"
	postgresUser "github.com/mot0x0/goth-api/internal/adapter/postgres/user"
	redisSession "github.com/mot0x0/goth-api/internal/adapter/redis/session"
	"github.com/mot0x0/goth-api/internal/config"
	"github.com/mot0x0/goth-api/internal/delivery/http"
	"github.com/mot0x0/goth-api/internal/delivery/http/handlers"
	"github.com/mot0x0/goth-api/internal/delivery/http/middleware"
	"github.com/mot0x0/goth-api/internal/domain/service"
	"github.com/mot0x0/goth-api/internal/domain/usecase/auth"
	"github.com/mot0x0/goth-api/internal/domain/usecase/session"
	"github.com/mot0x0/goth-api/internal/domain/usecase/user"
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
)

// HTTP providers
var HTTPSet = wire.NewSet(
	handlers.NewAuthHandler,
	handlers.NewUserHandler,
	handlers.NewSessionHandler,
	middleware.NewAuthMiddleware,
	http.NewServer,
)

// ProviderSet bundles everything
var ProviderSet = wire.NewSet(
	InfrastructureSet,
	RepositorySet,
	ServiceSet,
	UseCaseSet,
	HTTPSet,
)

func ProvideRedisClient(cfg *config.Config) *redis.Client {
	return redis.NewClient(&redis.Options{
		Addr:     cfg.RedisAddr,
		Password: cfg.RedisPassword,
		DB:       cfg.RedisDB,
	})
}
