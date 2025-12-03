package postgres

import (
	"context"

	"github.com/jmoiron/sqlx"
	"github.com/motixo/goth-api/internal/config"
	"github.com/motixo/goth-api/internal/domain/service"
	"github.com/motixo/goth-api/internal/domain/valueobject"
)

func SeedPermissions(db *sqlx.DB) error {
	adminPerm := valueobject.PermFullAccess
	adminRole := valueobject.RoleAdmin

	_, err := db.Exec(`
			INSERT INTO permissions (id, role_id, action, created_at)
			VALUES (gen_random_uuid(), $1, $2, CURRENT_TIMESTAMP AT TIME ZONE 'UTC')
			ON CONFLICT (role_id, action) DO NOTHING
		`, int8(adminRole), adminPerm)
	if err != nil {
		return err
	}

	return nil
}

func SeedAdminUser(db *sqlx.DB, passwordHasher service.PasswordHasher, cfg *config.Config) error {
	email := cfg.AdminEmail
	password := cfg.AdminPassword

	ctx := context.Background()
	hashedPassword, err := passwordHasher.Hash(ctx, password)
	if err != nil {
		return err
	}

	adminRole := valueobject.RoleAdmin
	activeStatus := valueobject.StatusActive

	_, err = db.Exec(`
		INSERT INTO users (id, email, password, status, role, created_at)
		VALUES (gen_random_uuid(), $1, $2, $3, $4, CURRENT_TIMESTAMP AT TIME ZONE 'UTC')
		ON CONFLICT (email) DO NOTHING
	`, email, hashedPassword.Value(), int8(activeStatus), int8(adminRole))

	if err != nil {
		return err
	}

	return nil
}
