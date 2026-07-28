package main

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/jmoiron/sqlx"
	"github.com/motixo/goat-api/internal/config"
	"github.com/motixo/goat-api/internal/domain/service"
	"github.com/motixo/goat-api/internal/domain/valueobject"
	authInfra "github.com/motixo/goat-api/internal/infra/auth"
	"github.com/motixo/goat-api/internal/infra/database/postgres"
	"github.com/motixo/goat-api/internal/pkg"
	"github.com/redis/go-redis/v9"
)

func TestInitializeAppStopsAfterPasswordHasherConstructionFailure(t *testing.T) {
	t.Parallel()

	recorder := &lifecycleRecorder{}
	dependencies := bootstrapDependencies{
		newLogger: func() (loggerResource, error) {
			recorder.append("logger.create")
			return loggerResource{
				logger: discardLogger{},
				sync:   recorder.action("logger.sync", nil),
			}, nil
		},
		newPostgres: func(context.Context, postgres.ClientConfig, pkg.Logger, service.PasswordHasher) (postgresResource, error) {
			t.Fatal("newPostgres() called after password hasher construction failed")
			return postgresResource{}, nil
		},
	}
	cfg := bootstrapTestConfig()
	cfg.PasswordHashMaxConcurrency = 0

	app, err := initializeApp(context.Background(), cfg, dependencies)
	if app != nil {
		t.Fatal("initializeApp() returned an application after password hasher construction failed")
	}
	if err == nil || !strings.Contains(err.Error(), "password hasher max concurrency") {
		t.Fatalf("initializeApp() error = %v, want password hasher configuration error", err)
	}
	want := []string{"logger.create", "logger.sync"}
	if got := recorder.snapshot(); !reflect.DeepEqual(got, want) {
		t.Fatalf("password hasher failure cleanup order = %v, want %v", got, want)
	}
}

func TestInitializeAppRollsBackPostgresWhenRedisConstructionFails(t *testing.T) {
	t.Parallel()

	redisErr := errors.New("redis unavailable")
	startupCtx, cancelStartup := context.WithCancel(context.Background())
	defer cancelStartup()
	recorder := &lifecycleRecorder{}
	dependencies := bootstrapDependencies{
		newLogger: func() (loggerResource, error) {
			recorder.append("logger.create")
			return loggerResource{
				logger: discardLogger{},
				sync:   recorder.action("logger.sync", nil),
			}, nil
		},
		newPostgres: func(context.Context, postgres.ClientConfig, pkg.Logger, service.PasswordHasher) (postgresResource, error) {
			recorder.append("postgres.create")
			return postgresResource{
				close: recorder.action("postgres.close", nil),
			}, nil
		},
		newRedis: func(ctx context.Context, _ *config.Config, _ pkg.Logger) (redisResource, error) {
			if ctx != startupCtx {
				t.Fatal("newRedis() did not receive the caller-owned startup context")
			}
			recorder.append("redis.create")
			return redisResource{}, redisErr
		},
		validateAssets: func(context.Context, *redis.Client) error {
			t.Fatal("validateAssets() called after Redis connection validation failed")
			return nil
		},
		buildRuntime: func(*config.Config, pkg.Logger, *sqlx.DB, *redis.Client, service.PasswordHasher) (runtimeResources, error) {
			t.Fatal("buildRuntime() called after Redis construction failed")
			return runtimeResources{}, nil
		},
	}

	app, err := initializeApp(startupCtx, bootstrapTestConfig(), dependencies)
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

func TestInitializeAppPreservesRedisStartupCancellationCause(t *testing.T) {
	t.Parallel()

	cancellationCause := errors.New("startup canceled during Redis validation")
	ctx, cancel := context.WithCancelCause(context.Background())
	recorder := &lifecycleRecorder{}
	dependencies := successfulBootstrapDependencies(recorder)
	dependencies.newRedis = func(
		startupCtx context.Context,
		_ *config.Config,
		_ pkg.Logger,
	) (redisResource, error) {
		recorder.append("redis.create")
		if startupCtx != ctx {
			t.Fatal("newRedis() did not receive the caller-owned startup context")
		}
		cancel(cancellationCause)
		<-startupCtx.Done()
		return redisResource{}, startupCtx.Err()
	}
	dependencies.validateAssets = func(context.Context, *redis.Client) error {
		t.Fatal("validateAssets() called after canceled Redis validation")
		return nil
	}
	dependencies.buildRuntime = func(
		*config.Config,
		pkg.Logger,
		*sqlx.DB,
		*redis.Client,
		service.PasswordHasher,
	) (runtimeResources, error) {
		t.Fatal("buildRuntime() called after canceled Redis validation")
		return runtimeResources{}, nil
	}

	app, err := initializeApp(ctx, bootstrapTestConfig(), dependencies)
	if app != nil {
		t.Fatal("initializeApp() returned an application after Redis startup cancellation")
	}
	for name, target := range map[string]error{
		"context cancellation": context.Canceled,
		"caller cause":         cancellationCause,
	} {
		if !errors.Is(err, target) {
			t.Errorf("initializeApp() error = %v, want %s", err, name)
		}
	}
	want := []string{
		"logger.create",
		"postgres.create",
		"redis.create",
		"postgres.close",
		"logger.sync",
	}
	if got := recorder.snapshot(); !reflect.DeepEqual(got, want) {
		t.Fatalf("startup cancellation order = %v, want %v", got, want)
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

	app, err := initializeApp(
		context.Background(),
		bootstrapTestConfig(),
		dependencies,
	)
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

func TestInitializeAppReceivesParsedConfigurationUnchanged(t *testing.T) {
	t.Parallel()

	const compositionPassword = "Composition1!"
	cfg := &config.Config{
		ServerPort:                 9090,
		PasswordPepper:             "bootstrap-config-pepper",
		PasswordHashMaxConcurrency: 2,
	}
	expectedHasher, err := authInfra.NewPasswordService(newPasswordHasherConfig(cfg))
	if err != nil {
		t.Fatalf("construct expected password hasher: %v", err)
	}
	expectedHash, err := expectedHasher.Hash(context.Background(), testPlainPassword(compositionPassword))
	if err != nil {
		t.Fatalf("create composition password fixture: %v", err)
	}
	want := *cfg
	recorder := &lifecycleRecorder{}
	dependencies := successfulBootstrapDependencies(recorder)
	newPostgres := dependencies.newPostgres
	wantPostgresConfig := newPostgresClientConfig(cfg)
	dependencies.newPostgres = func(
		ctx context.Context,
		got postgres.ClientConfig,
		logger pkg.Logger,
		passwordHasher service.PasswordHasher,
	) (postgresResource, error) {
		if got != wantPostgresConfig {
			t.Fatal("newPostgres() did not receive the mapped PostgreSQL client configuration")
		}
		verified, verifyErr := passwordHasher.Verify(ctx, testPlainPassword(compositionPassword), expectedHash)
		if verifyErr != nil {
			t.Fatalf("verify composition password: %v", verifyErr)
		}
		if !verified {
			t.Fatal("newPostgres() password hasher did not receive the configured password pepper")
		}
		return newPostgres(ctx, got, logger, passwordHasher)
	}
	newRedis := dependencies.newRedis
	dependencies.newRedis = func(
		ctx context.Context,
		got *config.Config,
		logger pkg.Logger,
	) (redisResource, error) {
		if got != cfg {
			t.Fatal("newRedis() did not receive the parsed configuration pointer")
		}
		return newRedis(ctx, got, logger)
	}
	dependencies.buildRuntime = func(
		got *config.Config,
		_ pkg.Logger,
		_ *sqlx.DB,
		_ *redis.Client,
		_ service.PasswordHasher,
	) (runtimeResources, error) {
		if got != cfg {
			t.Fatal("buildRuntime() did not receive the parsed configuration pointer")
		}
		recorder.append("runtime.create")
		return runtimeResources{}, nil
	}

	app, err := initializeApp(context.Background(), cfg, dependencies)
	if err != nil {
		t.Fatalf("initializeApp() error = %v", err)
	}
	if !reflect.DeepEqual(*cfg, want) {
		t.Fatalf("configuration changed during bootstrap: got %#v, want %#v", *cfg, want)
	}
	if app.address != ":9090" {
		t.Fatalf("application address = %q, want %q", app.address, ":9090")
	}
	if err := app.Shutdown(context.Background()); err != nil {
		t.Fatalf("Shutdown() error = %v", err)
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

	app, err := initializeApp(context.Background(), bootstrapTestConfig(), dependencies)
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

	app, err := initializeApp(context.Background(), bootstrapTestConfig(), dependencies)
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
	dependencies.newPostgres = func(context.Context, postgres.ClientConfig, pkg.Logger, service.PasswordHasher) (postgresResource, error) {
		recorder.append("postgres.create")
		return postgresResource{
			close: recorder.action("postgres.close", postgresCloseErr),
		}, nil
	}
	dependencies.newRedis = func(context.Context, *config.Config, pkg.Logger) (redisResource, error) {
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

	_, err := initializeApp(context.Background(), bootstrapTestConfig(), dependencies)
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

func TestInitializeAppStopsAfterPostgresFailureAndPreservesLoggerCleanupError(t *testing.T) {
	t.Parallel()

	postgresErr := errors.New("PostgreSQL startup failed")
	loggerSyncErr := errors.New("logger sync failed")
	recorder := &lifecycleRecorder{}
	dependencies := bootstrapDependencies{
		newLogger: func() (loggerResource, error) {
			recorder.append("logger.create")
			return loggerResource{
				logger: discardLogger{},
				sync:   recorder.action("logger.sync", loggerSyncErr),
			}, nil
		},
		newPostgres: func(
			ctx context.Context,
			_ postgres.ClientConfig,
			_ pkg.Logger,
			_ service.PasswordHasher,
		) (postgresResource, error) {
			recorder.append("postgres.create")
			if ctx == nil {
				t.Fatal("newPostgres() received a nil startup context")
			}
			return postgresResource{}, postgresErr
		},
		newRedis: func(context.Context, *config.Config, pkg.Logger) (redisResource, error) {
			recorder.append("UNEXPECTED:redis.create")
			return redisResource{}, nil
		},
		buildRuntime: func(*config.Config, pkg.Logger, *sqlx.DB, *redis.Client, service.PasswordHasher) (runtimeResources, error) {
			recorder.append("UNEXPECTED:runtime.create")
			return runtimeResources{}, nil
		},
	}

	app, err := initializeApp(context.Background(), bootstrapTestConfig(), dependencies)
	if app != nil {
		t.Fatal("initializeApp() returned an application after PostgreSQL failure")
	}
	if !errors.Is(err, postgresErr) {
		t.Fatalf("initializeApp() error = %v, want PostgreSQL startup error", err)
	}
	if !errors.Is(err, loggerSyncErr) {
		t.Fatalf("initializeApp() error = %v, want logger cleanup error", err)
	}
	want := []string{"logger.create", "postgres.create", "logger.sync"}
	if got := recorder.snapshot(); !reflect.DeepEqual(got, want) {
		t.Fatalf("startup events = %v, want %v", got, want)
	}
}

func TestInitializeAppCancellationAfterPostgresStopsRemainingConstruction(t *testing.T) {
	t.Parallel()

	cancellationCause := errors.New("startup canceled by process signal")
	ctx, cancel := context.WithCancelCause(context.Background())
	recorder := &lifecycleRecorder{}
	dependencies := successfulBootstrapDependencies(recorder)
	dependencies.newPostgres = func(
		startupCtx context.Context,
		_ postgres.ClientConfig,
		_ pkg.Logger,
		_ service.PasswordHasher,
	) (postgresResource, error) {
		recorder.append("postgres.create")
		if startupCtx != ctx {
			t.Fatal("newPostgres() did not receive the caller-owned startup context")
		}
		cancel(cancellationCause)
		return postgresResource{
			close: recorder.action("postgres.close", nil),
		}, nil
	}
	dependencies.newRedis = func(context.Context, *config.Config, pkg.Logger) (redisResource, error) {
		recorder.append("UNEXPECTED:redis.create")
		return redisResource{}, nil
	}

	app, err := initializeApp(ctx, bootstrapTestConfig(), dependencies)
	if app != nil {
		t.Fatal("initializeApp() returned an application after startup cancellation")
	}
	if !errors.Is(err, cancellationCause) {
		t.Fatalf("initializeApp() error = %v, want caller cancellation cause", err)
	}
	want := []string{
		"logger.create",
		"postgres.create",
		"postgres.close",
		"logger.sync",
	}
	if got := recorder.snapshot(); !reflect.DeepEqual(got, want) {
		t.Fatalf("startup cancellation order = %v, want %v", got, want)
	}
}

func TestInitializeAppAssetValidationInheritsStartupCancellation(t *testing.T) {
	t.Parallel()

	cancellationCause := errors.New("startup canceled during asset validation")
	ctx, cancel := context.WithCancelCause(context.Background())
	recorder := &lifecycleRecorder{}
	dependencies := successfulBootstrapDependencies(recorder)
	dependencies.validateAssets = func(validationCtx context.Context, _ *redis.Client) error {
		recorder.append("assets.validate")
		cancel(cancellationCause)
		<-validationCtx.Done()
		if context.Cause(validationCtx) != cancellationCause {
			t.Fatalf(
				"asset validation context cause = %v, want %v",
				context.Cause(validationCtx),
				cancellationCause,
			)
		}
		return validationCtx.Err()
	}

	app, err := initializeApp(ctx, bootstrapTestConfig(), dependencies)
	if app != nil {
		t.Fatal("initializeApp() returned an application after asset validation cancellation")
	}
	if !errors.Is(err, cancellationCause) {
		t.Fatalf("initializeApp() error = %v, want caller cancellation cause", err)
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
		t.Fatalf("asset validation cancellation order = %v, want %v", got, want)
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
		newPostgres: func(context.Context, postgres.ClientConfig, pkg.Logger, service.PasswordHasher) (postgresResource, error) {
			recorder.append("postgres.create")
			return postgresResource{
				close: recorder.action("postgres.close", nil),
			}, nil
		},
		newRedis: func(context.Context, *config.Config, pkg.Logger) (redisResource, error) {
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

func bootstrapTestConfig() *config.Config {
	return &config.Config{
		PasswordPepper:             "bootstrap-test-pepper",
		PasswordHashMaxConcurrency: 2,
	}
}

func testPlainPassword(raw string) valueobject.PlainPassword {
	password, err := valueobject.NewPlainPassword(raw)
	if err != nil {
		panic("test plaintext password is invalid")
	}
	return password
}
