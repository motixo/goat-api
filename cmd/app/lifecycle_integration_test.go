package main

import (
	"context"
	"errors"
	"net"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/motixo/goat-api/internal/config"
	"github.com/motixo/goat-api/internal/domain/service"
	"github.com/motixo/goat-api/internal/infra/database/postgres"
	"github.com/motixo/goat-api/internal/pkg"
	"github.com/redis/go-redis/v9"
)

func TestApplicationLifecycleIntegration(t *testing.T) {
	if os.Getenv("GOAT_LIFECYCLE_INTEGRATION") != "1" {
		t.Skip("set GOAT_LIFECYCLE_INTEGRATION=1 to run application lifecycle integration tests")
	}
	migrationConfig := lifecycleIntegrationConfig(t)
	migrationCtx, cancelMigration := context.WithTimeout(
		context.Background(),
		migrationConfig.DBInitializationTimeout,
	)
	defer cancelMigration()
	if _, err := postgres.Migrate(
		migrationCtx,
		newPostgresClientConfig(migrationConfig),
		discardLogger{},
	); err != nil {
		t.Fatalf("apply lifecycle integration migrations: %v", err)
	}

	t.Run("successful bootstrap and cleanup closes both stores", func(t *testing.T) {
		cfg := lifecycleIntegrationConfig(t)
		dependencies := defaultBootstrapDependencies()
		database, redisClient := captureIntegrationStores(&dependencies)

		app, err := initializeApp(context.Background(), cfg, dependencies)
		if err != nil {
			t.Fatalf("initializeApp() error = %v", err)
		}
		if got := database().DriverName(); got != "pgx" {
			t.Fatalf("PostgreSQL driver = %q, want pgx stdlib", got)
		}
		readinessChecker := newRuntimeReadinessChecker(database(), redisClient())
		readinessCtx, cancelReadiness := context.WithTimeout(context.Background(), 2*time.Second)
		if err := readinessChecker.CheckReadiness(readinessCtx); err != nil {
			cancelReadiness()
			t.Fatalf("live runtime readiness check error = %v", err)
		}
		cancelReadiness()
		t.Cleanup(func() {
			_ = app.Shutdown(context.Background())
		})

		runCtx, cancelRun := context.WithCancel(context.Background())
		defer cancelRun()
		serverReady := make(chan struct{})
		go cancelWhenServerStarts(runCtx, app, cancelRun, serverReady)

		if err := app.Run(runCtx); err != nil {
			t.Fatalf("Run() error = %v", err)
		}
		select {
		case <-serverReady:
		default:
			t.Fatal("application stopped before its HTTP server became ready")
		}
		if err := app.Shutdown(context.Background()); err != nil {
			t.Fatalf("repeated Shutdown() error = %v", err)
		}

		assertIntegrationStoresClosed(t, database(), redisClient())
		closedCtx, cancelClosedCheck := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancelClosedCheck()
		if err := readinessChecker.CheckReadiness(closedCtx); err == nil {
			t.Fatal("runtime readiness remained successful after dependency cleanup")
		}
	})

	t.Run("Redis failure rolls back an already-created PostgreSQL pool", func(t *testing.T) {
		cfg := lifecycleIntegrationConfig(t)
		cfg.RedisPort = 1
		dependencies := defaultBootstrapDependencies()
		database, _ := captureIntegrationStores(&dependencies)

		app, err := initializeApp(context.Background(), cfg, dependencies)
		if app != nil {
			t.Fatal("initializeApp() returned an application after Redis failure")
		}
		if err == nil {
			t.Fatal("initializeApp() error = nil, want Redis connection failure")
		}
		if database() == nil {
			t.Fatal("PostgreSQL was not constructed before the Redis failure")
		}
		if pingErr := database().PingContext(context.Background()); pingErr == nil {
			t.Fatal("PostgreSQL remained usable after Redis initialization failed")
		}
	})

	t.Run("runtime failure rolls back Redis before PostgreSQL", func(t *testing.T) {
		cfg := lifecycleIntegrationConfig(t)
		dependencies := defaultBootstrapDependencies()
		database, redisClient := captureIntegrationStores(&dependencies)
		runtimeErr := errors.New("injected runtime failure")
		dependencies.buildRuntime = func(
			*config.Config,
			pkg.Logger,
			*sqlx.DB,
			*redis.Client,
			service.PasswordHasher,
		) (runtimeResources, error) {
			return runtimeResources{}, runtimeErr
		}

		app, err := initializeApp(context.Background(), cfg, dependencies)
		if app != nil {
			t.Fatal("initializeApp() returned an application after runtime failure")
		}
		if !errors.Is(err, runtimeErr) {
			t.Fatalf("initializeApp() error = %v, want wrapped runtime error", err)
		}
		assertIntegrationStoresClosed(t, database(), redisClient())
	})
}

func lifecycleIntegrationConfig(t *testing.T) *config.Config {
	t.Helper()

	redisHost, redisPortValue, err := net.SplitHostPort(envOrDefault("GOAT_REDIS_ADDR", "127.0.0.1:6380"))
	if err != nil {
		t.Fatalf("parse GOAT_REDIS_ADDR: %v", err)
	}
	redisPort := parseIntegrationPort(t, "GOAT_REDIS_ADDR", redisPortValue)
	return &config.Config{
		Env:                        "development",
		ServerPort:                 0,
		HTTPReadHeaderTimeout:      5 * time.Second,
		HTTPReadTimeout:            15 * time.Second,
		HTTPWriteTimeout:           30 * time.Second,
		HTTPIdleTimeout:            time.Minute,
		HTTPReadinessTimeout:       2 * time.Second,
		HTTPMaxHeaderBytes:         64 << 10,
		HTTPMaxBodyBytes:           1 << 20,
		DBHost:                     envOrDefault("GOAT_POSTGRES_HOST", "127.0.0.1"),
		DBPort:                     parseIntegrationPort(t, "GOAT_POSTGRES_PORT", envOrDefault("GOAT_POSTGRES_PORT", "5432")),
		DBUser:                     envOrDefault("GOAT_POSTGRES_USER", "postgres"),
		DBPassword:                 envOrDefault("GOAT_POSTGRES_PASSWORD", "postgres"),
		DBName:                     envOrDefault("GOAT_POSTGRES_DB", "goat"),
		DBConnectionTimeout:        2 * time.Second,
		DBInitializationTimeout:    30 * time.Second,
		JWTSecret:                  "lifecycle-integration-secret",
		PasswordPepper:             "lifecycle-integration-pepper",
		PasswordHashMaxConcurrency: 2,
		RedisHost:                  redisHost,
		RedisPort:                  redisPort,
		RedisConnectionTimeout:     2 * time.Second,
		Seed:                       false,
		JWTExpiration:              15 * time.Minute,
		RefreshTokenExpiration:     24 * time.Hour,
		SessionExpiration:          30 * 24 * time.Hour,
		GinMode:                    "debug",
		RateLimitAuthLimit:         5,
		RateLimitAuthWindow:        time.Minute,
		RateLimitPublicLimit:       100,
		RateLimitPublicWindow:      time.Minute,
		RateLimitPrivateLimit:      60,
		RateLimitPrivateWindow:     time.Minute,
	}
}

func parseIntegrationPort(t *testing.T, name string, value string) uint16 {
	t.Helper()
	port, err := strconv.ParseUint(value, 10, 16)
	if err != nil || port == 0 {
		t.Fatalf("parse %s port: %q is not a valid port", name, value)
	}
	return uint16(port)
}

func captureIntegrationStores(
	dependencies *bootstrapDependencies,
) (func() *sqlx.DB, func() *redis.Client) {
	newPostgres := dependencies.newPostgres
	newRedis := dependencies.newRedis
	var database *sqlx.DB
	var redisClient *redis.Client

	dependencies.newPostgres = func(
		ctx context.Context,
		cfg postgres.ClientConfig,
		logger pkg.Logger,
		passwordHasher service.PasswordHasher,
	) (postgresResource, error) {
		resource, err := newPostgres(ctx, cfg, logger, passwordHasher)
		database = resource.db
		return resource, err
	}
	dependencies.newRedis = func(
		ctx context.Context,
		cfg *config.Config,
		logger pkg.Logger,
	) (redisResource, error) {
		resource, err := newRedis(ctx, cfg, logger)
		redisClient = resource.client
		return resource, err
	}
	return func() *sqlx.DB {
			return database
		}, func() *redis.Client {
			return redisClient
		}
}

func cancelWhenServerStarts(
	ctx context.Context,
	app *Application,
	cancel context.CancelFunc,
	ready chan<- struct{},
) {
	ticker := time.NewTicker(time.Millisecond)
	defer ticker.Stop()
	timeout := time.NewTimer(5 * time.Second)
	defer timeout.Stop()

	for {
		app.lifecycleMu.Lock()
		started := app.serverStarted
		app.lifecycleMu.Unlock()
		if started {
			close(ready)
			cancel()
			return
		}
		select {
		case <-ticker.C:
		case <-ctx.Done():
			return
		case <-timeout.C:
			cancel()
			return
		}
	}
}

func assertIntegrationStoresClosed(t *testing.T, database *sqlx.DB, redisClient *redis.Client) {
	t.Helper()
	if database == nil || redisClient == nil {
		t.Fatalf("captured stores = (%v, %v), want both resources", database, redisClient)
	}
	if err := database.PingContext(context.Background()); err == nil {
		t.Error("PostgreSQL remained usable after application cleanup")
	}
	if err := redisClient.Ping(context.Background()).Err(); err == nil {
		t.Error("Redis remained usable after application cleanup")
	}
}

func envOrDefault(name, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return fallback
}
