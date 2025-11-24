package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/jmoiron/sqlx"
	"github.com/mot0x0/gopi/internal/domain/entity"
)

type UserRepo struct {
	db *sqlx.DB
}

func NewUserRepository(db *sqlx.DB) *UserRepo {
	return &UserRepo{db: db}
}

func (r *UserRepo) Create(ctx context.Context, u *entity.User) error {
	query := `
        INSERT INTO users (id, email, password, status, created_at, updated_at)
        VALUES (:id, :email, :password, :status, :created_at, :updated_at)
    `
	_, err := r.db.NamedExecContext(ctx, query, u)
	return err
}

func (r *UserRepo) GetByID(ctx context.Context, id string) (*entity.User, error) {
	var user entity.User
	query := `
        SELECT id, email, status, created_at, updated_at
        FROM users
        WHERE id = $1
    `
	err := r.db.GetContext(ctx, &user, query, id)
	if err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *UserRepo) GetByEmail(ctx context.Context, email string) (*entity.User, error) {
	var user entity.User
	query := `
        SELECT id, email, password, status, created_at, updated_at
        FROM users
        WHERE email = $1
		LIMIT 1
    `
	err := r.db.GetContext(ctx, &user, query, email)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get user by email: %w", err)
	}
	return &user, nil
}
