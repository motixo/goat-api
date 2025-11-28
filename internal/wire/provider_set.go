package wire

import (
	"github.com/google/wire"
	"github.com/mot0x0/gopi/internal/adapter/postgres"
	postgresUser "github.com/mot0x0/gopi/internal/adapter/postgres/user"
	redisJTI "github.com/mot0x0/gopi/internal/adapter/redis/jti"
	redisSession "github.com/mot0x0/gopi/internal/adapter/redis/session"
	"github.com/mot0x0/gopi/internal/config"
	"github.com/mot0x0/gopi/internal/delivery/http"
	"github.com/mot0x0/gopi/internal/delivery/http/handlers"
	"github.com/mot0x0/gopi/internal/delivery/http/middleware"
	"github.com/mot0x0/gopi/internal/domain/service"
	"github.com/mot0x0/gopi/internal/domain/usecase/auth"
	"github.com/mot0x0/gopi/internal/domain/usecase/jti"
	"github.com/mot0x0/gopi/internal/domain/usecase/session"
	"github.com/mot0x0/gopi/internal/domain/usecase/user"
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
	redisJTI.NewRepository,
	redisSession.NewRepository,
)

// Service providers
var ServiceSet = wire.NewSet(
	service.NewULIDGenerator,
	service.NewPasswordService,
)

// Usecase providers
var UseCaseSet = wire.NewSet(
	jti.NewUsecase,
	auth.NewUsecase,
	session.NewUsecase,
	user.NewUsecase,
)

// HTTP providers
var HTTPSet = wire.NewSet(
	handlers.NewAuthHandler,
	handlers.NewUserHandler,
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
