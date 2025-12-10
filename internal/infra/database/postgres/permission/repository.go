package permission

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/jmoiron/sqlx"
	"github.com/motixo/goat-api/internal/domain/entity"
)

type Repository struct {
	db *sqlx.DB
}

func NewRepository(db *sqlx.DB) *Repository {
	return &Repository{db: db}
}

func (p *Repository) Create(ctx context.Context, u *entity.Permission) error {
	query := `
        INSERT INTO permissions (id, role_id, action, created_at)
        VALUES (:id, :role_id, :action, :created_at)
    `
	_, err := p.db.NamedExecContext(ctx, query, u)
	return err
}

func (r *Repository) GetByRoleID(ctx context.Context, roleID int8) ([]*entity.Permission, error) {
	var permission []*entity.Permission
	query := `
        SELECT id, role_id, action, created_at
        FROM permissions
        WHERE role_id = $1
    `
	err := r.db.SelectContext(ctx, &permission, query, roleID)
	if err != nil {
		return nil, err
	}
	return permission, nil
}

func (r *Repository) GetAll(ctx context.Context, offset, limit int) ([]*entity.Permission, int64, error) {
	var permission []*entity.Permission
	var total int64

	if err := r.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM permissions").Scan(&total); err != nil {
		return nil, 0, err
	}

	query := `
        SELECT id, role_id, action, created_at
        FROM permissions
		ORDER BY role_id DESC
		LIMIT $1 OFFSET $2
    `
	err := r.db.SelectContext(ctx, &permission, query, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	return permission, total, nil
}

func (r *Repository) Delete(ctx context.Context, permissionID string) (*int8, error) {
	var roleID int8
	err := r.db.QueryRowxContext(ctx, "DELETE FROM permissions WHERE id = $1 RETURNING role_id", permissionID).Scan(&roleID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("permission not found")
		}
		return nil, err
	}
	return &roleID, nil
}
