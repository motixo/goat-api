package postgres

import (
	"log"

	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
)

type Database struct {
	DB *sqlx.DB
}

func NewDatabase(dsn string) (*Database, error) {
	db, err := sqlx.Connect("postgres", dsn)
	if err != nil {
		return nil, err
	}

	schema := `
	CREATE TABLE IF NOT EXISTS users (
		id UUID PRIMARY KEY,
		email TEXT NOT NULL UNIQUE,
		password TEXT NOT NULL,
		status SMALLINT NOT NULL,
		created_at TIMESTAMP NOT NULL DEFAULT NOW(),
		updated_at TIMESTAMP NULL
	);`

	if _, err := db.Exec(schema); err != nil {
		return nil, err
	}

	log.Println("Database connected and users table ensured")

	return &Database{DB: db}, nil
}
