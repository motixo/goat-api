package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/jmoiron/sqlx"
	"github.com/motixo/goat-api/internal/pkg"
)

type migrationOperation func(
	context.Context,
	*sqlx.DB,
	[]migration,
) (MigrationResult, error)

// Migrate applies every pending embedded migration in one serialized
// PostgreSQL transaction. It is intended for the deployment migration command,
// not application runtime startup.
func Migrate(
	ctx context.Context,
	cfg ClientConfig,
	logger pkg.Logger,
) (MigrationResult, error) {
	return runMigrationOperation(ctx, cfg, logger, "apply", applyMigrations)
}

// ValidateMigrations checks that the database is exactly at the embedded
// migration version without performing schema writes.
func ValidateMigrations(
	ctx context.Context,
	cfg ClientConfig,
	logger pkg.Logger,
) (MigrationResult, error) {
	return runMigrationOperation(
		ctx,
		cfg,
		logger,
		"validate",
		func(
			operationCtx context.Context,
			db *sqlx.DB,
			available []migration,
		) (MigrationResult, error) {
			if err := validateCurrentMigrations(operationCtx, db, available); err != nil {
				return MigrationResult{}, err
			}
			return MigrationResult{
				CurrentVersion: available[len(available)-1].Version,
			}, nil
		},
	)
}

func runMigrationOperation(
	ctx context.Context,
	cfg ClientConfig,
	logger pkg.Logger,
	operationName string,
	operation migrationOperation,
) (result MigrationResult, err error) {
	if ctx == nil {
		return MigrationResult{}, errors.New("PostgreSQL migration context is required")
	}
	available, err := loadEmbeddedMigrations()
	if err != nil {
		return MigrationResult{}, err
	}
	db, err := newDatabasePool(cfg)
	if err != nil {
		return MigrationResult{}, err
	}
	defer func() {
		if closeErr := db.Close(); closeErr != nil {
			err = errors.Join(err, fmt.Errorf("close PostgreSQL migration connection: %w", closeErr))
		}
	}()

	if err := pingDatabase(ctx, cfg.ConnectionTimeout, db); err != nil {
		logger.Error("failed to connect to database for migrations", "error", err)
		return MigrationResult{}, err
	}
	operationCtx, cancelOperation := context.WithTimeout(ctx, cfg.InitializationTimeout)
	defer cancelOperation()

	result, err = operation(operationCtx, db, available)
	if err != nil {
		logger.Error("PostgreSQL migration operation failed", "operation", operationName, "error", err)
		return MigrationResult{}, err
	}
	return result, nil
}
