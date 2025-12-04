package postgres

import (
	"context"

	"github.com/jmoiron/sqlx"
	"github.com/motixo/goth-api/internal/config"
	"github.com/motixo/goth-api/internal/domain/service"
	"github.com/motixo/goth-api/internal/domain/valueobject"
)

func SeedPermissions(db *sqlx.DB) error {

	adminRole := int8(valueobject.RoleAdmin)
	clientRole := int8(valueobject.RoleClient)
	operatorRole := int8(valueobject.RoleOperator)

	adminPerm := valueobject.PermFullAccess

	clientPerm := []valueobject.Permission{
		valueobject.PermUserRead,
		valueobject.PermUserUpdate,
		valueobject.PermUserDelete,
		valueobject.PermSessionRead,
		valueobject.PermSessionDelete,
	}

	operatorPerm := []valueobject.Permission{
		valueobject.PermUserRead,
		valueobject.PermUserUpdate,
		valueobject.PermUserChangeStatus,
		valueobject.PermSessionRead,
	}

	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	insertStmt := `
    INSERT INTO permissions (id, role_id, action, created_at)
    VALUES (gen_random_uuid(), $1, $2, CURRENT_TIMESTAMP AT TIME ZONE 'UTC')
    ON CONFLICT (role_id, action) DO NOTHING;
	`

	// Admin
	_, err = tx.Exec(insertStmt, adminRole, adminPerm)
	if err != nil {
		return err
	}

	// Client
	for _, p := range clientPerm {
		_, err = tx.Exec(insertStmt, clientRole, p)
		if err != nil {
			return err
		}
	}

	// Operator
	for _, p := range operatorPerm {
		_, err = tx.Exec(insertStmt, operatorRole, p)
		if err != nil {
			return err
		}
	}

	return tx.Commit()
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
