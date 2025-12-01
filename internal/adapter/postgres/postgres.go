package postgres

import (
	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
	"github.com/mot0x0/goth-api/internal/config"
	"github.com/mot0x0/goth-api/internal/domain/service"
)

func NewDatabase(cfg *config.Config, logger service.Logger) (*sqlx.DB, error) {
	db, err := sqlx.Connect("postgres", cfg.DSN())
	if err != nil {
		logger.Error("failed to connect to database", "error", err)
		return nil, err
	}

	schema := `
	CREATE TABLE IF NOT EXISTS users (
		id UUID PRIMARY KEY,
		email TEXT NOT NULL UNIQUE,
		password TEXT NOT NULL,
		status SMALLINT NOT NULL,
		role SMALLINT NOT NULL,
		created_at TIMESTAMP NOT NULL DEFAULT NOW(),
		updated_at TIMESTAMP NULL
	);`

	if _, err := db.Exec(schema); err != nil {
		logger.Error("failed to ensure users table", "error", err)
		return nil, err
	}

	logger.Info("Database connected and users table ensured")

	return sqlx.Connect("postgres", cfg.DSN())
}
