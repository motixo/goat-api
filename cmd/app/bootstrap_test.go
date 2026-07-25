package main

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/jmoiron/sqlx"
	"github.com/motixo/goat-api/internal/config"
	"github.com/motixo/goat-api/internal/domain/service"
	"github.com/motixo/goat-api/internal/pkg"
	"github.com/redis/go-redis/v9"
)

func TestInitializeAppRollsBackPostgresWhenRedisConstructionFails(t *testing.T) {
	t.Parallel()

	redisErr := errors.New("redis unavailable")
	recorder := &lifecycleRecorder{}
	dependencies := bootstrapDependencies{
		newLogger: func() (loggerResource, error) {
			recorder.append("logger.create")
			return loggerResource{
				logger: discardLogger{},
				sync:   recorder.action("logger.sync", nil),
			}, nil
		},
		newPostgres: func(*config.Config, pkg.Logger, service.PasswordHasher) (postgresResource, error) {
			recorder.append("postgres.create")
			return postgresResource{
				close: recorder.action("postgres.close", nil),
			}, nil
		},
		newRedis: func(*config.Config, pkg.Logger) (redisResource, error) {
			recorder.append("redis.create")
			return redisResource{}, redisErr
		},
		buildRuntime: func(*config.Config, pkg.Logger, *sqlx.DB, *redis.Client, service.PasswordHasher) (runtimeResources, error) {
			t.Fatal("buildRuntime() called after Redis construction failed")
			return runtimeResources{}, nil
		},
	}

	app, err := initializeApp(&config.Config{}, dependencies)
	if app != nil {
		t.Fatal("initializeApp() returned an application after Redis failure")
	}
	if !errors.Is(err, redisErr) {
		t.Fatalf("initializeApp() error = %v, want wrapped Redis error", err)
	}

	want := []string{
		"logger.create",
		"postgres.create",
		"redis.create",
		"postgres.close",
		"logger.sync",
	}
	if got := recorder.snapshot(); !reflect.DeepEqual(got, want) {
		t.Fatalf("rollback order = %v, want %v", got, want)
	}
}

func TestInitializeAppSuccessfulBootstrapAndCleanup(t *testing.T) {
	t.Parallel()

	recorder := &lifecycleRecorder{}
	dependencies := successfulBootstrapDependencies(recorder)
	dependencies.buildRuntime = func(*config.Config, pkg.Logger, *sqlx.DB, *redis.Client, service.PasswordHasher) (runtimeResources, error) {
		recorder.append("runtime.create")
		return runtimeResources{
			server:  newFakeLifecycleServer(recorder),
			cleaner: &fakeLifecycleWorker{recorder: recorder},
		}, nil
	}

	app, err := initializeApp(&config.Config{ServerPort: "127.0.0.1:0"}, dependencies)
	if err != nil {
		t.Fatalf("initializeApp() error = %v", err)
	}
	if err := app.Shutdown(context.Background()); err != nil {
		t.Fatalf("Shutdown() error = %v", err)
	}
	if err := app.Shutdown(context.Background()); err != nil {
		t.Fatalf("second Shutdown() error = %v", err)
	}

	want := []string{
		"logger.create",
		"postgres.create",
		"redis.create",
		"assets.validate",
		"runtime.create",
		"redis.close",
		"postgres.close",
		"logger.sync",
	}
	if got := recorder.snapshot(); !reflect.DeepEqual(got, want) {
		t.Fatalf("lifecycle order = %v, want %v", got, want)
	}
}

func TestInitializeAppRollsBackRedisPostgresAndLoggerAfterRuntimeFailure(t *testing.T) {
	t.Parallel()

	runtimeErr := errors.New("runtime construction failed")
	recorder := &lifecycleRecorder{}
	dependencies := successfulBootstrapDependencies(recorder)
	dependencies.buildRuntime = func(*config.Config, pkg.Logger, *sqlx.DB, *redis.Client, service.PasswordHasher) (runtimeResources, error) {
		recorder.append("runtime.create")
		return runtimeResources{}, runtimeErr
	}

	app, err := initializeApp(&config.Config{}, dependencies)
	if app != nil {
		t.Fatal("initializeApp() returned an application after runtime failure")
	}
	if !errors.Is(err, runtimeErr) {
		t.Fatalf("initializeApp() error = %v, want wrapped runtime error", err)
	}

	want := []string{
		"logger.create",
		"postgres.create",
		"redis.create",
		"assets.validate",
		"runtime.create",
		"redis.close",
		"postgres.close",
		"logger.sync",
	}
	if got := recorder.snapshot(); !reflect.DeepEqual(got, want) {
		t.Fatalf("rollback order = %v, want %v", got, want)
	}
}

func TestInitializeAppValidatesAssetsBeforeRuntimeAndRollsBackInReverseOrder(t *testing.T) {
	t.Parallel()

	validationErr := errors.New("embedded runtime assets are invalid")
	recorder := &lifecycleRecorder{}
	dependencies := successfulBootstrapDependencies(recorder)
	dependencies.validateAssets = func(context.Context, *redis.Client) error {
		recorder.append("assets.validate")
		return validationErr
	}
	dependencies.buildRuntime = func(*config.Config, pkg.Logger, *sqlx.DB, *redis.Client, service.PasswordHasher) (runtimeResources, error) {
		recorder.append("UNEXPECTED:runtime.create")
		return runtimeResources{}, nil
	}

	app, err := initializeApp(&config.Config{}, dependencies)
	if app != nil {
		t.Fatal("initializeApp() returned an application after asset validation failure")
	}
	if !errors.Is(err, validationErr) {
		t.Fatalf("initializeApp() error = %v, want wrapped validation error", err)
	}

	want := []string{
		"logger.create",
		"postgres.create",
		"redis.create",
		"assets.validate",
		"redis.close",
		"postgres.close",
		"logger.sync",
	}
	if got := recorder.snapshot(); !reflect.DeepEqual(got, want) {
		t.Fatalf("rollback order = %v, want %v", got, want)
	}
}

func TestInitializeAppPreservesPrimaryAndRollbackErrors(t *testing.T) {
	t.Parallel()

	redisErr := errors.New("redis unavailable")
	postgresCloseErr := errors.New("postgres close failed")
	loggerSyncErr := errors.New("logger sync failed")
	recorder := &lifecycleRecorder{}
	dependencies := successfulBootstrapDependencies(recorder)
	dependencies.newPostgres = func(*config.Config, pkg.Logger, service.PasswordHasher) (postgresResource, error) {
		recorder.append("postgres.create")
		return postgresResource{
			close: recorder.action("postgres.close", postgresCloseErr),
		}, nil
	}
	dependencies.newRedis = func(*config.Config, pkg.Logger) (redisResource, error) {
		recorder.append("redis.create")
		return redisResource{}, redisErr
	}
	dependencies.newLogger = func() (loggerResource, error) {
		recorder.append("logger.create")
		return loggerResource{
			logger: discardLogger{},
			sync:   recorder.action("logger.sync", loggerSyncErr),
		}, nil
	}

	_, err := initializeApp(&config.Config{}, dependencies)
	for name, target := range map[string]error{
		"Redis construction": redisErr,
		"PostgreSQL close":   postgresCloseErr,
		"logger sync":        loggerSyncErr,
	} {
		if !errors.Is(err, target) {
			t.Errorf("initializeApp() error = %v, want wrapped %s error", err, name)
		}
	}
}

func successfulBootstrapDependencies(recorder *lifecycleRecorder) bootstrapDependencies {
	return bootstrapDependencies{
		newLogger: func() (loggerResource, error) {
			recorder.append("logger.create")
			return loggerResource{
				logger: discardLogger{},
				sync:   recorder.action("logger.sync", nil),
			}, nil
		},
		newPostgres: func(*config.Config, pkg.Logger, service.PasswordHasher) (postgresResource, error) {
			recorder.append("postgres.create")
			return postgresResource{
				close: recorder.action("postgres.close", nil),
			}, nil
		},
		newRedis: func(*config.Config, pkg.Logger) (redisResource, error) {
			recorder.append("redis.create")
			return redisResource{
				close: recorder.action("redis.close", nil),
			}, nil
		},
		validateAssets: func(context.Context, *redis.Client) error {
			recorder.append("assets.validate")
			return nil
		},
		buildRuntime: func(*config.Config, pkg.Logger, *sqlx.DB, *redis.Client, service.PasswordHasher) (runtimeResources, error) {
			recorder.append("runtime.create")
			return runtimeResources{}, nil
		},
	}
}
