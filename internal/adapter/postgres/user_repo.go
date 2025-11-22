package postgres

import (
	"context"

	"github.com/jmoiron/sqlx"
	"github.com/mtextr/gopi/internal/domain"
)

type UserRepo struct {
	db *sqlx.DB
}

func NewUserRepo(db *sqlx.DB) *UserRepo {
	return &UserRepo{db: db}
}

func (r *UserRepo) Create(ctx context.Context, u *domain.User) error {
	query := `
        INSERT INTO users (id, email, password, status, created_at, updated_at)
        VALUES (:id, :email, :password, :status, :created_at, :updated_at)
    `
	_, err := r.db.NamedExecContext(ctx, query, u)
	return err
}

func (r *UserRepo) GetByID(ctx context.Context, id string) (*domain.User, error) {
	var user domain.User
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
