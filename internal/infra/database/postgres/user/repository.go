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
	userdetail "github.com/motixo/goat-api/internal/usecase/user/detail"
	useremailchange "github.com/motixo/goat-api/internal/usecase/user/emailchange"
	userlisting "github.com/motixo/goat-api/internal/usecase/user/listing"
	userrolechange "github.com/motixo/goat-api/internal/usecase/user/rolechange"
	userstatuschange "github.com/motixo/goat-api/internal/usecase/user/statuschange"
	userupdating "github.com/motixo/goat-api/internal/usecase/user/updating"
)

type Repository struct {
	db *sqlx.DB
}

var (
	_ userdetail.UserDetailReader               = (*Repository)(nil)
	_ useremailchange.UserEmailUpdateWriter     = (*Repository)(nil)
	_ userlisting.UserListReader                = (*Repository)(nil)
	_ userrolechange.UserRoleUpdateWriter       = (*Repository)(nil)
	_ userstatuschange.UserStatusSnapshotReader = (*Repository)(nil)
	_ userupdating.UserUpdateWriter             = (*Repository)(nil)
)

const (
	userDetailSelectQuery         = `SELECT id, email, role, status, created_at FROM users WHERE id = $1 LIMIT 1`
	userStatusSnapshotSelectQuery = `SELECT id, role, status FROM users WHERE id = $1 LIMIT 1`
	userListSelectFields          = `SELECT id, email, role, status, created_at FROM users`
	uniqueViolationSQLState       = "23505"
	userEmailUniqueConstraint     = "users_email_key"
)

func NewRepository(db *sqlx.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) Create(ctx context.Context, u *entity.User) error {
	if u.CredentialVersion <= 0 {
		return fmt.Errorf("credential version must be positive")
	}
	if u.PasswordDigest.IsZero() {
		return fmt.Errorf("user password digest is required")
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
	return row.toDomain()
}

func (r *Repository) FindDetailByID(
	ctx context.Context,
	id string,
) (userdetail.UserDetail, error) {
	var row userDetailRow
	if err := r.db.GetContext(ctx, &row, userDetailSelectQuery, id); err != nil {
		return userdetail.UserDetail{}, translateUserFindByIDError(err)
	}
	return row.toDetail(), nil
}

func (r *Repository) FindStatusSnapshotByID(
	ctx context.Context,
	id string,
) (userstatuschange.UserStatusSnapshot, error) {
	var row userStatusSnapshotRow
	if err := r.db.GetContext(ctx, &row, userStatusSnapshotSelectQuery, id); err != nil {
		return userstatuschange.UserStatusSnapshot{}, translateUserFindByIDError(err)
	}
	return row.toStatusSnapshot(), nil
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
	return row.toDomain()
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
	digest valueobject.PasswordDigest,
) (int64, error) {
	if digest.IsZero() {
		return 0, fmt.Errorf("user password digest is required")
	}
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
		digest.Encoded(),
		time.Now().UTC(),
		id,
	)
	if err != nil {
		return 0, translateUserFindByIDError(err)
	}
	return version, nil
}

func (r *Repository) UpdateUser(
	ctx context.Context,
	command userupdating.UserUpdateCommand,
) error {
	return r.updateUserFields(
		ctx,
		command.UserID,
		command.Email,
		command.PasswordDigest,
	)
}

func (r *Repository) UpdateEmail(
	ctx context.Context,
	command useremailchange.UserEmailUpdateCommand,
) error {
	return r.updateUserFields(
		ctx,
		command.UserID,
		command.Email,
		valueobject.PasswordDigest{},
	)
}

func (r *Repository) UpdateRole(
	ctx context.Context,
	command userrolechange.UserRoleUpdateCommand,
) (userrolechange.UserRoleUpdateResult, error) {
	if !command.ExpectedRole.IsKnown() || !command.RequestedRole.IsKnown() {
		return userrolechange.UserRoleUpdateResult{}, domainErrors.ErrBadRequest
	}

	var storedRole int16
	err := r.db.GetContext(
		ctx,
		&storedRole,
		`UPDATE users
		 SET role = $1,
		     updated_at = $2
		 WHERE id = $3
		   AND role = $4
		 RETURNING role`,
		int16(command.RequestedRole),
		time.Now().UTC(),
		command.UserID,
		int16(command.ExpectedRole),
	)
	if err == nil {
		current := valueobject.UserRole(storedRole)
		if !current.IsKnown() || current != command.RequestedRole {
			return userrolechange.UserRoleUpdateResult{}, fmt.Errorf(
				"compare-and-set user role returned an invalid role",
			)
		}
		return userrolechange.UserRoleUpdateResult{
			Outcome:     userrolechange.UserRoleUpdateApplied,
			CurrentRole: current,
		}, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return userrolechange.UserRoleUpdateResult{}, fmt.Errorf(
			"compare-and-set user role: %w",
			translateUserWriteError(err),
		)
	}

	return r.classifyRoleUpdateMiss(
		ctx,
		command.UserID,
		command.ExpectedRole,
		command.RequestedRole,
	)
}

func (r *Repository) classifyRoleUpdateMiss(
	ctx context.Context,
	userID string,
	expectedRole valueobject.UserRole,
	requestedRole valueobject.UserRole,
) (userrolechange.UserRoleUpdateResult, error) {
	var storedRole int16
	err := r.db.GetContext(
		ctx,
		&storedRole,
		"SELECT role FROM users WHERE id = $1",
		userID,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return userrolechange.UserRoleUpdateResult{
			Outcome: userrolechange.UserRoleUpdateNotFound,
		}, nil
	}
	if err != nil {
		return userrolechange.UserRoleUpdateResult{}, fmt.Errorf(
			"classify user role update: %w",
			err,
		)
	}

	current := valueobject.UserRole(storedRole)
	if !current.IsKnown() {
		return userrolechange.UserRoleUpdateResult{}, fmt.Errorf(
			"authoritative user role is invalid",
		)
	}
	if current == requestedRole {
		return userrolechange.UserRoleUpdateResult{
			Outcome:     userrolechange.UserRoleUpdateAlreadyApplied,
			CurrentRole: current,
		}, nil
	}
	if current == expectedRole {
		return userrolechange.UserRoleUpdateResult{}, fmt.Errorf(
			"compare-and-set user role missed while expected role remains current",
		)
	}
	return userrolechange.UserRoleUpdateResult{
		Outcome:     userrolechange.UserRoleUpdateConflict,
		CurrentRole: current,
	}, nil
}

func (r *Repository) updateUserFields(
	ctx context.Context,
	userID string,
	email string,
	passwordDigest valueobject.PasswordDigest,
) error {
	setClauses := []string{}
	args := []interface{}{}
	argIndex := 1

	if email != "" {
		setClauses = append(setClauses, fmt.Sprintf("email = $%d", argIndex))
		args = append(args, email)
		argIndex++
	}

	if !passwordDigest.IsZero() {
		setClauses = append(setClauses, fmt.Sprintf("password = $%d", argIndex))
		args = append(args, passwordDigest.Encoded())
		argIndex++
		setClauses = append(setClauses, "credential_version = credential_version + 1")
	}

	if len(setClauses) == 0 {
		return domainErrors.ErrBadRequest
	}

	setClauses = append(setClauses, fmt.Sprintf("updated_at = $%d", argIndex))
	args = append(args, time.Now().UTC())
	argIndex++

	setClausesStr := strings.Join(setClauses, ", ")
	query := fmt.Sprintf("UPDATE users SET %s WHERE id = $%d", setClausesStr, argIndex)
	args = append(args, userID)

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

func (r *Repository) ListUsers(
	ctx context.Context,
	offset int,
	limit int,
	criteria userlisting.UserListCriteria,
) (userlisting.UserListResult, error) {
	countFields := `SELECT COUNT(*) FROM users`
	whereClauses := []string{}
	args := []interface{}{}
	argIndex := 1

	if len(criteria.Roles) > 0 {
		whereClauses = append(whereClauses, fmt.Sprintf("role = ANY($%d)", argIndex))
		args = append(args, userRolesToDatabase(criteria.Roles))
		argIndex++
	}

	// Status filter
	if len(criteria.Statuses) > 0 {
		whereClauses = append(whereClauses, fmt.Sprintf("status = ANY($%d)", argIndex))
		args = append(args, userStatusesToDatabase(criteria.Statuses))
		argIndex++
	}

	// Search filter (email)
	if criteria.Search != "" {
		whereClauses = append(whereClauses, fmt.Sprintf("email ILIKE $%d", argIndex))
		args = append(args, "%"+criteria.Search+"%")
		argIndex++
	}

	var whereClause string
	if len(whereClauses) > 0 {
		whereClause = " WHERE " + strings.Join(whereClauses, " AND ")
	}

	countQuery := countFields + whereClause
	var total int64
	if err := r.db.GetContext(ctx, &total, countQuery, args...); err != nil {
		return userlisting.UserListResult{}, err
	}

	if total == 0 {
		return userlisting.UserListResult{Items: []userlisting.UserListItem{}}, nil
	}

	selectQuery := buildUserListSelectQuery(whereClause, argIndex)
	args = append(args, limit, offset)

	var rows []userListRow
	if err := r.db.SelectContext(ctx, &rows, selectQuery, args...); err != nil {
		return userlisting.UserListResult{}, err
	}

	return userlisting.UserListResult{
		Items: userListRowsToItems(rows),
		Total: total,
	}, nil
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
