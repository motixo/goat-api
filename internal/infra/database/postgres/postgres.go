package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/stdlib"
	"github.com/jmoiron/sqlx"
	"github.com/motixo/goat-api/internal/domain/service"
	"github.com/motixo/goat-api/internal/pkg"
)

const (
	driverName = "pgx"
)

type startupPinger interface {
	PingContext(context.Context) error
}

type databaseCloser interface {
	Close() error
}

func NewDatabase(
	ctx context.Context,
	cfg ClientConfig,
	logger pkg.Logger,
	passwordSrv service.PasswordHasher,
) (*sqlx.DB, error) {
	if ctx == nil {
		return nil, errors.New("PostgreSQL startup context is required")
	}
	migrations, err := loadEmbeddedMigrations()
	if err != nil {
		return nil, err
	}
	if cfg.Seed && passwordSrv == nil {
		return nil, errors.New("PostgreSQL administrator seed password hasher is required")
	}

	db, err := newDatabasePool(cfg)
	if err != nil {
		return nil, err
	}
	if err := pingDatabase(ctx, cfg.ConnectionTimeout, db); err != nil {
		logger.Error("failed to connect to database", "error", err)
		return nil, closeFailedDatabaseInitialization(db, err)
	}

	initializationCtx, cancelInitialization := context.WithTimeout(
		ctx,
		cfg.InitializationTimeout,
	)
	defer cancelInitialization()
	if err := validateCurrentMigrations(initializationCtx, db, migrations); err != nil {
		logger.Error("failed to validate database migrations", "error", err)
		return nil, closeFailedDatabaseInitialization(db, err)
	}

	logger.Info("Database connected and schema migration state validated")

	if cfg.Seed {
		if err := SeedPermissions(initializationCtx, db); err != nil {
			logger.Error("failed to seed permissions", "error", err)
			return nil, closeFailedDatabaseInitialization(db, err)
		}
		logger.Info("Permissions seeded successfully")

		if err := SeedAdminUser(
			initializationCtx,
			db,
			passwordSrv,
			cfg.AdminEmail,
			cfg.AdminPassword,
		); err != nil {
			logger.Error("failed to seed admin user", "error", err)
			return nil, closeFailedDatabaseInitialization(db, err)
		}
		logger.Info("admin user seeded successfully")
	}
	return db, nil
}

func newDatabasePool(cfg ClientConfig) (*sqlx.DB, error) {
	connectionConfig, err := newConnectionConfig(cfg)
	if err != nil {
		return nil, err
	}
	return sqlx.NewDb(stdlib.OpenDB(*connectionConfig), driverName), nil
}

func pingDatabase(
	ctx context.Context,
	timeout time.Duration,
	pinger startupPinger,
) error {
	pingCtx, cancelPing := context.WithTimeout(ctx, timeout)
	err := pinger.PingContext(pingCtx)
	if err != nil {
		err = startupOperationError("validate PostgreSQL connection", pingCtx, err)
	}
	cancelPing()
	return err
}

func startupOperationError(operation string, ctx context.Context, err error) error {
	operationErr := fmt.Errorf("%s: %w", operation, err)
	cause := context.Cause(ctx)
	if cause == nil || errors.Is(operationErr, cause) {
		return operationErr
	}
	return errors.Join(
		operationErr,
		fmt.Errorf("%s context: %w", operation, cause),
	)
}

func closeFailedDatabaseInitialization(db databaseCloser, initializationErr error) error {
	if closeErr := db.Close(); closeErr != nil {
		return errors.Join(
			initializationErr,
			fmt.Errorf("close PostgreSQL after initialization failure: %w", closeErr),
		)
	}
	return initializationErr
}
