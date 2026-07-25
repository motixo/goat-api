package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/jmoiron/sqlx"
	"github.com/motixo/goat-api/internal/config"
	"github.com/motixo/goat-api/internal/domain/service"
	"github.com/motixo/goat-api/internal/domain/valueobject"
)

func SeedPermissions(db *sqlx.DB) (err error) {

	adminRole := int8(valueobject.RoleAdmin)
	clientRole := int8(valueobject.RoleClient)
	operatorRole := int8(valueobject.RoleOperator)

	adminPerm := valueobject.PermFullAccess

	clientPerm := []valueobject.Permission{}

	operatorPerm := []valueobject.Permission{
		valueobject.PermUserRead,
		valueobject.PermUserUpdate,
		valueobject.PermUserChangeStatus,
	}

	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer func() {
		if err == nil {
			return
		}
		rollbackErr := tx.Rollback()
		if rollbackErr != nil && !errors.Is(rollbackErr, sql.ErrTxDone) {
			err = errors.Join(err, fmt.Errorf("rollback permission seed: %w", rollbackErr))
		}
	}()

	insertStmt := `
    INSERT INTO permissions (id, role, action, created_at)
    VALUES (gen_random_uuid(), $1, $2, CURRENT_TIMESTAMP AT TIME ZONE 'UTC')
    ON CONFLICT (role, action) DO NOTHING;
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

	err = tx.Commit()
	return err
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
	`, email, hashedPassword, int8(activeStatus), int8(adminRole))

	if err != nil {
		return err
	}

	return nil
}
