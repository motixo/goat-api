package permission

import (
	"context"

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
