package postgres

import (
	"errors"
	"fmt"

	"github.com/jmoiron/sqlx"
	"github.com/motixo/goat-api/internal/config"
	"github.com/motixo/goat-api/internal/domain/service"
	"github.com/motixo/goat-api/internal/pkg"
)

func NewDatabase(cfg *config.Config, logger pkg.Logger, passwordSrv service.PasswordHasher) (*sqlx.DB, error) {
	db, err := sqlx.Open("postgres", cfg.DSN())
	if err != nil {
		logger.Error("failed to open database", "error", err)
		return nil, err
	}
	if err := db.Ping(); err != nil {
		logger.Error("failed to connect to database", "error", err)
		return nil, closeFailedDatabaseInitialization(db, err)
	}

	userSchema := `
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

	permissionSchema := `
	CREATE TABLE IF NOT EXISTS permissions (
		id UUID PRIMARY KEY,
		role SMALLINT NOT NULL,
		action TEXT NOT NULL,
		created_at TIMESTAMP NOT NULL DEFAULT NOW(),
		updated_at TIMESTAMP NULL,
		CONSTRAINT unique_role_action UNIQUE(role, action)
	);`

	if _, err := db.Exec(userSchema); err != nil {
		logger.Error("failed to ensure users table", "error", err)
		return nil, closeFailedDatabaseInitialization(db, err)
	}
	if _, err := db.Exec(`
	CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_users_created_at_desc 
	ON users (created_at DESC);
	`); err != nil {
		logger.Error("failed to index user table", "error", err)
		return nil, closeFailedDatabaseInitialization(db, err)
	}

	if _, err := db.Exec(permissionSchema); err != nil {
		logger.Error("failed to ensure permissions table", "error", err)
		return nil, closeFailedDatabaseInitialization(db, err)
	}

	logger.Info("Database connected and users, permissions table ensured")

	if cfg.Seed == 1 {
		if err := SeedPermissions(db); err != nil {
			logger.Error("failed to seed permissions", "error", err)
			return nil, closeFailedDatabaseInitialization(db, err)
		}
		logger.Info("Permissions seeded successfully")

		if err := SeedAdminUser(db, passwordSrv, cfg); err != nil {
			logger.Error("failed to seed admin user", "error", err)
			return nil, closeFailedDatabaseInitialization(db, err)
		}
		logger.Info("admin user seeded successfully")
	}
	return db, err
}

func closeFailedDatabaseInitialization(db *sqlx.DB, initializationErr error) error {
	if closeErr := db.Close(); closeErr != nil {
		return errors.Join(
			initializationErr,
			fmt.Errorf("close PostgreSQL after initialization failure: %w", closeErr),
		)
	}
	return initializationErr
}
