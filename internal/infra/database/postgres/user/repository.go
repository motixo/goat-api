package user

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/motixo/goat-api/internal/domain/entity"
	domanErrors "github.com/motixo/goat-api/internal/domain/errors"
	"github.com/motixo/goat-api/internal/domain/repository"
	"github.com/motixo/goat-api/internal/domain/repository/dto"
)

type Repository struct {
	db *sqlx.DB
}

func NewRepository(db *sqlx.DB) repository.UserRepository {
	return &Repository{db: db}
}

func (r *Repository) Create(ctx context.Context, u *entity.User) error {
	query := `
        INSERT INTO users (id, email, password, status, role, created_at, updated_at)
        VALUES (:id, :email, :password, :status, :role, :created_at, :updated_at)
    `
	_, err := r.db.NamedExecContext(ctx, query, u)
	return err
}

func (r *Repository) FindByID(ctx context.Context, id string) (*entity.User, error) {
	var user entity.User
	query := `
        SELECT id, email, password, status, role, created_at, updated_at
        FROM users
        WHERE id = $1
		LIMIT 1
    `
	err := r.db.GetContext(ctx, &user, query, id)
	if err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *Repository) FindByEmail(ctx context.Context, email string) (*entity.User, error) {
	var user entity.User
	query := `
        SELECT id, email, password, status, role, created_at, updated_at
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

func (r *Repository) Update(ctx context.Context, userID string, u dto.UserUpdate) error {
	setClauses := []string{}
	args := map[string]interface{}{
		"id":         userID,
		"updated_at": time.Now(),
	}

	if u.Email != nil {
		setClauses = append(setClauses, "email = :email")
		args["email"] = *u.Email
	}
	if u.Password != nil {
		setClauses = append(setClauses, "password = :password")
		args["password"] = *u.Password
	}
	if u.Status != nil {
		setClauses = append(setClauses, "status = :status")
		args["status"] = *u.Status
	}
	if u.Role != nil {
		setClauses = append(setClauses, "role = :role")
		args["role"] = *u.Role
	}

	setClauses = append(setClauses, "updated_at = :updated_at")

	query := fmt.Sprintf("UPDATE users SET %s WHERE id = :id", strings.Join(setClauses, ", "))
	result, err := r.db.NamedExecContext(ctx, query, args)
	if err != nil {
		return err
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if affected == 0 {
		return domanErrors.ErrUserNotFound
	}

	return nil
}

func (r *Repository) Delete(ctx context.Context, userID string) error {
	_, err := r.db.ExecContext(ctx, "DELETE FROM users WHERE id = $1", userID)
	return err
}

func (r *Repository) List(ctx context.Context, offset, limit int) ([]*entity.User, int64, error) {
	var users []*entity.User
	var countErr, dataErr error

	var total int64

	countErr = r.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM users").Scan(&total)

	query := `
		SELECT id, email, role, status, created_at
		FROM users
		ORDER BY created_at DESC
		LIMIT $1 OFFSET $2
	`
	dataErr = r.db.SelectContext(ctx, &users, query, limit, offset)
	if countErr != nil {
		return nil, 0, countErr
	}
	if dataErr != nil {
		return nil, 0, dataErr
	}

	return users, total, nil
}
