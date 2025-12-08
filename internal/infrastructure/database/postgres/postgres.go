package postgres

import (
	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
	"github.com/motixo/goat-api/internal/config"
	"github.com/motixo/goat-api/internal/domain/service"
	"github.com/motixo/goat-api/internal/infrastructure/logger"
)

func NewDatabase(cfg *config.Config, logger logger.Logger, passwordSrv service.PasswordHasher) (*sqlx.DB, error) {
	db, err := sqlx.Connect("postgres", cfg.DSN())
	if err != nil {
		logger.Error("failed to connect to database", "error", err)
		return nil, err
	}

	userSchema := `
	CREATE TABLE IF NOT EXISTS users (
		id UUID PRIMARY KEY,
		email TEXT NOT NULL UNIQUE,
		password TEXT NOT NULL,
		status SMALLINT NOT NULL,
		role SMALLINT NOT NULL,
		created_at TIMESTAMP NOT NULL DEFAULT NOW(),
		updated_at TIMESTAMP NULL
	);`

	permissionSchema := `
	CREATE TABLE IF NOT EXISTS permissions (
		id UUID PRIMARY KEY,
		role_id SMALLINT NOT NULL,
		action TEXT NOT NULL,
		created_at TIMESTAMP NOT NULL DEFAULT NOW(),
		updated_at TIMESTAMP NULL,
		CONSTRAINT unique_role_action UNIQUE(role_id, action)
	);`

	if _, err := db.Exec(userSchema); err != nil {
		logger.Error("failed to ensure users table", "error", err)
		return nil, err
	}

	if _, err := db.Exec(permissionSchema); err != nil {
		logger.Error("failed to ensure permissions table", "error", err)
		return nil, err
	}

	logger.Info("Database connected and users, permissions table ensured")

	if cfg.Seed == 1 {
		if err := SeedPermissions(db); err != nil {
			logger.Error("failed to seed permissions", "error", err)
			return nil, err
		}
		logger.Info("Permissions seeded successfully")

		if err := SeedAdminUser(db, passwordSrv, cfg); err != nil {
			logger.Error("failed to seed admin user", "error", err)
			return nil, err
		}
		logger.Info("admin user seeded successfully")
	}
	return sqlx.Connect("postgres", cfg.DSN())
}
