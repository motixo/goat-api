package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/jmoiron/sqlx"
	"github.com/motixo/goat-api/internal/domain/service"
	"github.com/motixo/goat-api/internal/domain/valueobject"
)

func SeedPermissions(ctx context.Context, db *sqlx.DB) (err error) {
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

	tx, err := db.BeginTxx(ctx, nil)
	if err != nil {
		return startupOperationError("begin permission seed transaction", ctx, err)
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
	_, err = tx.ExecContext(ctx, insertStmt, adminRole, adminPerm)
	if err != nil {
		return startupOperationError("seed admin permission", ctx, err)
	}

	// Client
	for _, p := range clientPerm {
		_, err = tx.ExecContext(ctx, insertStmt, clientRole, p)
		if err != nil {
			return startupOperationError("seed client permission", ctx, err)
		}
	}

	// Operator
	for _, p := range operatorPerm {
		_, err = tx.ExecContext(ctx, insertStmt, operatorRole, p)
		if err != nil {
			return startupOperationError("seed operator permission", ctx, err)
		}
	}

	err = tx.Commit()
	if err != nil {
		return startupOperationError("commit permission seed transaction", ctx, err)
	}
	return err
}

func SeedAdminUser(
	ctx context.Context,
	db *sqlx.DB,
	passwordHasher service.PasswordHasher,
	email string,
	password string,
) error {
	plainPassword, err := valueobject.NewPlainPassword(password)
	if err != nil {
		return startupOperationError("validate seeded admin password", ctx, err)
	}
	digest, err := passwordHasher.Hash(ctx, plainPassword)
	if err != nil {
		return startupOperationError("hash seeded admin password", ctx, err)
	}

	adminRole := valueobject.RoleAdmin
	activeStatus := valueobject.StatusActive

	_, err = db.ExecContext(ctx, `
		INSERT INTO users (id, email, password, status, role, created_at)
		VALUES (gen_random_uuid(), $1, $2, $3, $4, CURRENT_TIMESTAMP AT TIME ZONE 'UTC')
		ON CONFLICT (email) DO NOTHING
	`, email, digest.Encoded(), int8(activeStatus), int8(adminRole))

	if err != nil {
		return startupOperationError("seed admin user", ctx, err)
	}

	return nil
}
