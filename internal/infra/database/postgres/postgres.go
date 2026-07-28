package postgres

import (
	"context"
	"database/sql"
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

	userSchema = `
	CREATE TABLE IF NOT EXISTS users (
		id UUID PRIMARY KEY,
		email TEXT NOT NULL UNIQUE,
		password TEXT NOT NULL,
		status SMALLINT NOT NULL,
		role SMALLINT NOT NULL,
		credential_version BIGINT NOT NULL DEFAULT 1 CHECK (credential_version > 0),
		created_at TIMESTAMP NOT NULL DEFAULT NOW(),
		updated_at TIMESTAMP NULL
	);`

	userCreatedAtIndex = `
	CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_users_created_at_desc
	ON users (created_at DESC);
	`

	permissionSchema = `
	CREATE TABLE IF NOT EXISTS permissions (
		id UUID PRIMARY KEY,
		role SMALLINT NOT NULL,
		action TEXT NOT NULL,
		created_at TIMESTAMP NOT NULL DEFAULT NOW(),
		updated_at TIMESTAMP NULL,
		CONSTRAINT unique_role_action UNIQUE(role, action)
	);`
)

type startupPinger interface {
	PingContext(context.Context) error
}

type startupExecutor interface {
	ExecContext(context.Context, string, ...any) (sql.Result, error)
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
	connectionConfig, err := newConnectionConfig(cfg)
	if err != nil {
		return nil, err
	}
	if cfg.Seed && passwordSrv == nil {
		return nil, errors.New("PostgreSQL administrator seed password hasher is required")
	}

	db := sqlx.NewDb(stdlib.OpenDB(*connectionConfig), driverName)
	if err := pingDatabase(ctx, cfg.ConnectionTimeout, db); err != nil {
		logger.Error("failed to connect to database", "error", err)
		return nil, closeFailedDatabaseInitialization(db, err)
	}

	initializationCtx, cancelInitialization := context.WithTimeout(
		ctx,
		cfg.InitializationTimeout,
	)
	defer cancelInitialization()
	if err := initializeSchema(initializationCtx, db); err != nil {
		logger.Error("failed to initialize database schema", "error", err)
		return nil, closeFailedDatabaseInitialization(db, err)
	}

	logger.Info("Database connected and users, permissions table ensured")

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

func initializeSchema(ctx context.Context, executor startupExecutor) error {
	operations := []struct {
		name      string
		statement string
	}{
		{name: "ensure users table", statement: userSchema},
		{name: "ensure users created-at index", statement: userCreatedAtIndex},
		{name: "ensure permissions table", statement: permissionSchema},
	}
	for _, operation := range operations {
		if _, err := executor.ExecContext(ctx, operation.statement); err != nil {
			return startupOperationError(operation.name, ctx, err)
		}
	}
	return nil
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
