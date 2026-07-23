package permission

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/jmoiron/sqlx"
	"github.com/motixo/goat-api/internal/domain/entity"
	"github.com/motixo/goat-api/internal/domain/repository"
	"github.com/motixo/goat-api/internal/domain/valueobject"
)

type Repository struct {
	db *sqlx.DB
}

func NewRepository(db *sqlx.DB) repository.PermissionRepository {
	return &Repository{db: db}
}

func (r *Repository) Create(ctx context.Context, permission *entity.Permission) error {
	query := `
        INSERT INTO permissions (id, role, action, created_at)
        VALUES (:id, :role, :action, :created_at)
    `
	_, err := r.db.NamedExecContext(ctx, query, permissionRowFromDomain(permission))
	return err
}

func (r *Repository) List(ctx context.Context, offset, limit int) ([]*entity.Permission, int64, error) {
	var rows []permissionRow
	var total int64

	if err := r.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM permissions").Scan(&total); err != nil {
		return nil, 0, err
	}

	query := `
        SELECT id, role, action, created_at
        FROM permissions
		ORDER BY role DESC
		LIMIT $1 OFFSET $2
    `
	err := r.db.SelectContext(ctx, &rows, query, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	return permissionRowsToDomain(rows), total, nil
}

func (r *Repository) GetByRoleID(ctx context.Context, role valueobject.UserRole) ([]*entity.Permission, error) {
	var rows []permissionRow
	query := `
        SELECT id, role, action, created_at
        FROM permissions
        WHERE role = $1
    `
	err := r.db.SelectContext(ctx, &rows, query, int8(role))
	if err != nil {
		return nil, err
	}
	return permissionRowsToDomain(rows), nil
}

func (r *Repository) Delete(ctx context.Context, permissionID string) (int8, error) {
	var roleID int8
	err := r.db.QueryRowxContext(ctx, "DELETE FROM permissions WHERE id = $1 RETURNING role", permissionID).Scan(&roleID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return 0, fmt.Errorf("permission not found")
		}
		return 0, err
	}
	return roleID, nil
}
