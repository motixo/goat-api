package user

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/lib/pq"
	"github.com/motixo/goat-api/internal/domain/entity"
	domainErrors "github.com/motixo/goat-api/internal/domain/errors"
	"github.com/motixo/goat-api/internal/domain/repository"
	"github.com/motixo/goat-api/internal/domain/valueobject"
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

func (r *Repository) Update(ctx context.Context, user *entity.User) error {
	setClauses := []string{}
	args := []interface{}{}
	argIndex := 1

	if user.Email != "" {
		setClauses = append(setClauses, fmt.Sprintf("email = $%d", argIndex))
		args = append(args, user.Email)
		argIndex++
	}

	if !user.Password.IsZero() {
		setClauses = append(setClauses, fmt.Sprintf("password = $%d", argIndex))
		args = append(args, user.Password)
		argIndex++
	}

	if user.Role != valueobject.RoleUnknown {
		setClauses = append(setClauses, fmt.Sprintf("role = $%d", argIndex))
		args = append(args, user.Role)
		argIndex++
	}

	if user.Status != valueobject.StatusUnknown {
		setClauses = append(setClauses, fmt.Sprintf("status = $%d", argIndex))
		args = append(args, user.Status)
		argIndex++
	}

	if len(setClauses) == 0 {
		return domainErrors.ErrBadRequest
	}

	setClauses = append(setClauses, fmt.Sprintf("updated_at = $%d", argIndex))
	args = append(args, time.Now())
	argIndex++

	setClausesStr := strings.Join(setClauses, ", ")
	query := fmt.Sprintf("UPDATE users SET %s WHERE id = $%d", setClausesStr, argIndex)
	args = append(args, user.ID)

	result, err := r.db.ExecContext(ctx, query, args...)
	if err != nil {
		return err
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if affected == 0 {
		return domainErrors.ErrUserNotFound
	}

	return nil
}

func (r *Repository) Delete(ctx context.Context, userID string) error {
	_, err := r.db.ExecContext(ctx, "DELETE FROM users WHERE id = $1", userID)
	return err
}

func (r *Repository) List(ctx context.Context, offset, limit int, filters entity.UserFilter) ([]*entity.User, int64, error) {
	selectFields := `SELECT id, email, role, status, created_at, updated_at FROM users`
	countFields := `SELECT COUNT(*) FROM users`
	whereClauses := []string{}
	args := []interface{}{}
	argIndex := 1

	if len(filters.Roles) > 0 {
		whereClauses = append(whereClauses, fmt.Sprintf("role = ANY($%d)", argIndex))
		args = append(args, pq.Array(filters.Roles))
		argIndex++
	}

	// Status filter
	if len(filters.Statuses) > 0 {
		whereClauses = append(whereClauses, fmt.Sprintf("status = ANY($%d)", argIndex))
		args = append(args, pq.Array(filters.Statuses))
		argIndex++
	}

	// Search filter (email)
	if filters.Search != "" {
		whereClauses = append(whereClauses, fmt.Sprintf("email ILIKE $%d", argIndex))
		args = append(args, "%"+filters.Search+"%")
		argIndex++
	}

	var whereClause string
	if len(whereClauses) > 0 {
		whereClause = " WHERE " + strings.Join(whereClauses, " AND ")
	}

	countQuery := countFields + whereClause
	var total int64
	if err := r.db.GetContext(ctx, &total, countQuery, args...); err != nil {
		return nil, 0, err
	}

	if total == 0 {
		return []*entity.User{}, 0, nil
	}

	selectQuery := selectFields + whereClause + " ORDER BY created_at DESC"
	selectQuery += fmt.Sprintf(" LIMIT $%d OFFSET $%d", argIndex, argIndex+1)
	args = append(args, limit, offset)

	var users []*entity.User
	if err := r.db.SelectContext(ctx, &users, selectQuery, args...); err != nil {
		return nil, 0, err
	}

	return users, total, nil
}
