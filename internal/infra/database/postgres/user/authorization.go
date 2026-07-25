package user

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/motixo/goat-api/internal/domain/valueobject"
	"github.com/motixo/goat-api/internal/usecase/authorization"
)

const getSecurityStateQuery = `
	SELECT
		u.id,
		u.status,
		u.role,
		u.credential_version,
		COALESCE(
			array_agg(p.action ORDER BY p.action)
				FILTER (WHERE p.action IS NOT NULL),
			ARRAY[]::text[]
		) AS permissions
	FROM users AS u
	LEFT JOIN permissions AS p ON p.role = u.role
	WHERE u.id = $1
	GROUP BY u.id, u.status, u.role, u.credential_version
`

const getIdentityStateQuery = `
	SELECT id, status, credential_version
	FROM users
	WHERE id = $1
`

type identityStateRow struct {
	UserID            string `db:"id"`
	Status            int16  `db:"status"`
	CredentialVersion int64  `db:"credential_version"`
}

type authorizationStateRow struct {
	UserID            string            `db:"id"`
	Status            int16             `db:"status"`
	Role              int16             `db:"role"`
	CredentialVersion int64             `db:"credential_version"`
	Permissions       postgresTextArray `db:"permissions"`
}

type postgresTextArray []string

func (array *postgresTextArray) Scan(source any) error {
	var values []string
	if err := pgtype.NewMap().SQLScanner(&values).Scan(source); err != nil {
		return fmt.Errorf("scan PostgreSQL text array: %w", err)
	}
	*array = values
	return nil
}

func (r *Repository) GetIdentityState(
	ctx context.Context,
	userID string,
) (authorization.IdentityState, error) {
	var row identityStateRow
	if err := r.db.GetContext(ctx, &row, getIdentityStateQuery, userID); err != nil {
		return authorization.IdentityState{}, translateUserFindByIDError(err)
	}
	return row.toIdentityState()
}

func (row identityStateRow) toIdentityState() (authorization.IdentityState, error) {
	if row.UserID == "" {
		return authorization.IdentityState{}, fmt.Errorf("identity-state user ID is empty")
	}
	if row.CredentialVersion <= 0 {
		return authorization.IdentityState{}, fmt.Errorf(
			"identity-state credential version must be positive",
		)
	}
	status := valueobject.UserStatus(row.Status)
	switch status {
	case valueobject.StatusInactive, valueobject.StatusActive, valueobject.StatusSuspended:
	default:
		return authorization.IdentityState{}, fmt.Errorf("identity-state status is invalid")
	}
	return authorization.IdentityState{
		UserID:            row.UserID,
		Status:            status,
		CredentialVersion: row.CredentialVersion,
	}, nil
}

func (r *Repository) GetSecurityState(
	ctx context.Context,
	userID string,
) (authorization.SecurityState, error) {
	var row authorizationStateRow
	if err := r.db.GetContext(ctx, &row, getSecurityStateQuery, userID); err != nil {
		return authorization.SecurityState{}, translateUserFindByIDError(err)
	}
	return row.toSecurityState()
}

func (row authorizationStateRow) toSecurityState() (authorization.SecurityState, error) {
	if row.UserID == "" {
		return authorization.SecurityState{}, fmt.Errorf("security-state user ID is empty")
	}
	if row.CredentialVersion <= 0 {
		return authorization.SecurityState{}, fmt.Errorf("security-state credential version must be positive")
	}

	role := valueobject.UserRole(row.Role)
	switch role {
	case valueobject.RoleClient, valueobject.RoleOperator, valueobject.RoleAdmin:
	default:
		return authorization.SecurityState{}, fmt.Errorf("security-state role is invalid")
	}
	status := valueobject.UserStatus(row.Status)
	switch status {
	case valueobject.StatusInactive, valueobject.StatusActive, valueobject.StatusSuspended:
	default:
		return authorization.SecurityState{}, fmt.Errorf("security-state status is invalid")
	}

	permissions := make([]valueobject.Permission, len(row.Permissions))
	for index := range row.Permissions {
		permission, err := valueobject.ParsePermission(row.Permissions[index])
		if err != nil {
			return authorization.SecurityState{}, fmt.Errorf(
				"security-state permission is invalid: %w",
				err,
			)
		}
		permissions[index] = permission
	}
	permissionSet, err := valueobject.NewPermissionSet(permissions)
	if err != nil {
		return authorization.SecurityState{}, fmt.Errorf(
			"normalize security-state permissions: %w",
			err,
		)
	}

	return authorization.SecurityState{
		UserID:            row.UserID,
		Status:            status,
		Role:              role,
		CredentialVersion: row.CredentialVersion,
		Permissions:       permissionSet,
	}, nil
}

var _ authorization.SecurityStateReader = (*Repository)(nil)
var _ authorization.IdentityStateReader = (*Repository)(nil)
