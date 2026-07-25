package user

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jmoiron/sqlx"
	"github.com/motixo/goat-api/internal/domain/entity"
	domainErrors "github.com/motixo/goat-api/internal/domain/errors"
	"github.com/motixo/goat-api/internal/domain/repository"
	"github.com/motixo/goat-api/internal/domain/valueobject"
)

type Repository struct {
	db *sqlx.DB
}

const (
	userListSelectFields      = `SELECT id, email, role, status, credential_version, created_at, updated_at FROM users`
	uniqueViolationSQLState   = "23505"
	userEmailUniqueConstraint = "users_email_key"
)

func NewRepository(db *sqlx.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) Create(ctx context.Context, u *entity.User) error {
	if u.CredentialVersion <= 0 {
		return fmt.Errorf("credential version must be positive")
	}
	query := `
        INSERT INTO users (id, email, password, status, role, credential_version, created_at, updated_at)
        VALUES (:id, :email, :password, :status, :role, :credential_version, :created_at, :updated_at)
	`
	_, err := r.db.NamedExecContext(ctx, query, userRowFromDomain(u))
	return translateUserWriteError(err)
}

func (r *Repository) ExistsByID(ctx context.Context, id string) (bool, error) {
	var exists bool
	if err := r.db.GetContext(ctx, &exists, "SELECT EXISTS (SELECT 1 FROM users WHERE id = $1)", id); err != nil {
		return false, err
	}
	return exists, nil
}

func (r *Repository) FindByID(ctx context.Context, id string) (*entity.User, error) {
	var row userRow
	query := `
        SELECT id, email, password, status, role, credential_version, created_at, updated_at
        FROM users
        WHERE id = $1
		LIMIT 1
	`
	err := r.db.GetContext(ctx, &row, query, id)
	if err != nil {
		return nil, translateUserFindByIDError(err)
	}
	return row.toDomain(), nil
}

func (r *Repository) FindByEmail(ctx context.Context, email string) (*entity.User, error) {
	var row userRow
	query := `
        SELECT id, email, password, status, role, credential_version, created_at, updated_at
        FROM users
        WHERE email = $1
		LIMIT 1
    `
	err := r.db.GetContext(ctx, &row, query, email)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get user by email: %w", err)
	}
	return row.toDomain(), nil
}

func (r *Repository) GetCredentialVersion(ctx context.Context, id string) (int64, error) {
	var version int64
	err := r.db.GetContext(ctx, &version, "SELECT credential_version FROM users WHERE id = $1", id)
	if err != nil {
		return 0, translateUserFindByIDError(err)
	}
	if version <= 0 {
		return 0, fmt.Errorf("user %s has invalid credential version", id)
	}
	return version, nil
}

func (r *Repository) UpdatePassword(
	ctx context.Context,
	id string,
	password valueobject.Password,
) (int64, error) {
	var version int64
	err := r.db.GetContext(
		ctx,
		&version,
		`UPDATE users
		 SET password = $1,
		     credential_version = credential_version + 1,
		     updated_at = $2
		 WHERE id = $3
		 RETURNING credential_version`,
		password.Encoded(),
		time.Now().UTC(),
		id,
	)
	if err != nil {
		return 0, translateUserFindByIDError(err)
	}
	return version, nil
}

func (r *Repository) Update(ctx context.Context, user *entity.User) error {
	row := userRowFromDomain(user)
	setClauses := []string{}
	args := []interface{}{}
	argIndex := 1

	if row.Email != "" {
		setClauses = append(setClauses, fmt.Sprintf("email = $%d", argIndex))
		args = append(args, row.Email)
		argIndex++
	}

	if row.PasswordHash != "" {
		setClauses = append(setClauses, fmt.Sprintf("password = $%d", argIndex))
		args = append(args, row.PasswordHash)
		argIndex++
		setClauses = append(setClauses, "credential_version = credential_version + 1")
	}

	if row.Role != int16(valueobject.RoleUnknown) {
		setClauses = append(setClauses, fmt.Sprintf("role = $%d", argIndex))
		args = append(args, row.Role)
		argIndex++
	}

	if len(setClauses) == 0 {
		return domainErrors.ErrBadRequest
	}

	setClauses = append(setClauses, fmt.Sprintf("updated_at = $%d", argIndex))
	args = append(args, time.Now().UTC())
	argIndex++

	setClausesStr := strings.Join(setClauses, ", ")
	query := fmt.Sprintf("UPDATE users SET %s WHERE id = $%d", setClausesStr, argIndex)
	args = append(args, row.ID)

	result, err := r.db.ExecContext(ctx, query, args...)
	if err != nil {
		return translateUserWriteError(err)
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

func (r *Repository) UpdateStatus(
	ctx context.Context,
	userID string,
	expected valueobject.UserStatus,
	requested valueobject.UserStatus,
) (repository.UserStatusUpdateResult, error) {
	if !expected.IsKnown() {
		return repository.UserStatusUpdateResult{}, fmt.Errorf(
			"expected user status is invalid",
		)
	}
	if !requested.IsKnown() {
		return repository.UserStatusUpdateResult{}, fmt.Errorf(
			"requested user status is invalid",
		)
	}
	if expected == requested {
		return repository.UserStatusUpdateResult{}, fmt.Errorf(
			"compare-and-set user statuses must differ",
		)
	}

	var storedStatus int16
	err := r.db.GetContext(
		ctx,
		&storedStatus,
		`UPDATE users
		 SET status = $1,
		     updated_at = $2
		 WHERE id = $3
		   AND status = $4
		 RETURNING status`,
		int16(requested),
		time.Now().UTC(),
		userID,
		int16(expected),
	)
	if err == nil {
		return repository.UserStatusUpdateResult{
			Outcome:       repository.UserStatusUpdateApplied,
			CurrentStatus: valueobject.UserStatus(storedStatus),
		}, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return repository.UserStatusUpdateResult{}, fmt.Errorf(
			"compare-and-set user status: %w",
			err,
		)
	}

	return r.classifyStatusUpdateMiss(ctx, userID, requested)
}

func (r *Repository) classifyStatusUpdateMiss(
	ctx context.Context,
	userID string,
	requested valueobject.UserStatus,
) (repository.UserStatusUpdateResult, error) {
	var storedStatus int16
	err := r.db.GetContext(
		ctx,
		&storedStatus,
		"SELECT status FROM users WHERE id = $1",
		userID,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return repository.UserStatusUpdateResult{
			Outcome: repository.UserStatusUpdateNotFound,
		}, nil
	}
	if err != nil {
		return repository.UserStatusUpdateResult{}, fmt.Errorf(
			"classify user status update: %w",
			err,
		)
	}

	current := valueobject.UserStatus(storedStatus)
	if !current.IsKnown() {
		return repository.UserStatusUpdateResult{}, fmt.Errorf(
			"authoritative user status is invalid",
		)
	}
	if current == requested {
		return repository.UserStatusUpdateResult{
			Outcome:       repository.UserStatusUpdateAlreadyApplied,
			CurrentStatus: current,
		}, nil
	}
	return repository.UserStatusUpdateResult{
		Outcome:       repository.UserStatusUpdateConflict,
		CurrentStatus: current,
	}, nil
}

func (r *Repository) Delete(ctx context.Context, userID string) error {
	result, err := r.db.ExecContext(ctx, "DELETE FROM users WHERE id = $1", userID)
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

func (r *Repository) List(ctx context.Context, offset, limit int, filters repository.UserListFilter) ([]*entity.User, int64, error) {
	countFields := `SELECT COUNT(*) FROM users`
	whereClauses := []string{}
	args := []interface{}{}
	argIndex := 1

	if len(filters.Roles) > 0 {
		whereClauses = append(whereClauses, fmt.Sprintf("role = ANY($%d)", argIndex))
		args = append(args, userRolesToDatabase(filters.Roles))
		argIndex++
	}

	// Status filter
	if len(filters.Statuses) > 0 {
		whereClauses = append(whereClauses, fmt.Sprintf("status = ANY($%d)", argIndex))
		args = append(args, userStatusesToDatabase(filters.Statuses))
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

	selectQuery := buildUserListSelectQuery(whereClause, argIndex)
	args = append(args, limit, offset)

	var rows []userRow
	if err := r.db.SelectContext(ctx, &rows, selectQuery, args...); err != nil {
		return nil, 0, err
	}

	return userRowsToDomain(rows), total, nil
}

func buildUserListSelectQuery(whereClause string, argIndex int) string {
	return userListSelectFields + whereClause +
		" ORDER BY created_at DESC, id DESC" +
		fmt.Sprintf(" LIMIT $%d OFFSET $%d", argIndex, argIndex+1)
}

func userRolesToDatabase(roles []valueobject.UserRole) []int16 {
	values := make([]int16, len(roles))
	for index := range roles {
		values[index] = int16(roles[index])
	}
	return values
}

func userStatusesToDatabase(statuses []valueobject.UserStatus) []int16 {
	values := make([]int16, len(statuses))
	for index := range statuses {
		values[index] = int16(statuses[index])
	}
	return values
}

func translateUserFindByIDError(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("%w: %w", domainErrors.ErrUserNotFound, err)
	}
	return err
}

func translateUserWriteError(err error) error {
	if err == nil {
		return nil
	}

	var postgresErr *pgconn.PgError
	if errors.As(err, &postgresErr) &&
		postgresErr.Code == uniqueViolationSQLState &&
		postgresErr.ConstraintName == userEmailUniqueConstraint {
		return fmt.Errorf("%w: %w", domainErrors.ErrEmailAlreadyExists, err)
	}

	return err
}
