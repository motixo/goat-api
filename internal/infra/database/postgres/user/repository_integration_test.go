package user

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/jmoiron/sqlx"
	"github.com/motixo/goat-api/internal/domain/entity"
	domainErrors "github.com/motixo/goat-api/internal/domain/errors"
	"github.com/motixo/goat-api/internal/domain/repository"
	"github.com/motixo/goat-api/internal/domain/valueobject"
	useremailchange "github.com/motixo/goat-api/internal/usecase/user/emailchange"
	userlisting "github.com/motixo/goat-api/internal/usecase/user/listing"
	userrolechange "github.com/motixo/goat-api/internal/usecase/user/rolechange"
	userupdating "github.com/motixo/goat-api/internal/usecase/user/updating"
)

func TestRepositoryIntegrationLoadsAuthoritativeSecurityStateInOneProjection(t *testing.T) {
	repo := newPostgresUserIntegrationRepository(t)
	ctx := context.Background()
	user := integrationUser(
		"40000000-0000-4000-8000-000000000001",
		"authorization@example.com",
		valueobject.RoleOperator,
		valueobject.StatusActive,
		time.Now().UTC(),
	)
	if err := repo.Create(ctx, user); err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if _, err := repo.db.ExecContext(ctx, `
		INSERT INTO permissions (id, role, action) VALUES
			('40000000-0000-4000-8000-000000000010', $1, $2),
			('40000000-0000-4000-8000-000000000011', $1, $3)
	`, int16(valueobject.RoleOperator), valueobject.PermUserUpdate, valueobject.PermUserRead); err != nil {
		t.Fatalf("seed operator permissions: %v", err)
	}

	state, err := repo.GetSecurityState(ctx, user.ID)
	if err != nil {
		t.Fatalf("GetSecurityState() error = %v", err)
	}
	if state.UserID != user.ID ||
		state.Status != valueobject.StatusActive ||
		state.Role != valueobject.RoleOperator ||
		state.CredentialVersion != entity.InitialCredentialVersion {
		t.Fatalf("security state = %#v, want current user projection", state)
	}
	wantPermissions := []valueobject.Permission{
		valueobject.PermUserRead,
		valueobject.PermUserUpdate,
	}
	if got := state.Permissions.Values(); !reflect.DeepEqual(got, wantPermissions) {
		t.Fatalf("security permissions = %v, want %v", got, wantPermissions)
	}

	identityState, err := repo.GetIdentityState(ctx, user.ID)
	if err != nil {
		t.Fatalf("GetIdentityState() error = %v", err)
	}
	if identityState.UserID != user.ID ||
		identityState.Status != valueobject.StatusActive ||
		identityState.CredentialVersion != entity.InitialCredentialVersion {
		t.Fatalf("identity state = %#v, want current user projection", identityState)
	}

	_, err = repo.GetSecurityState(ctx, "49999999-9999-4999-8999-999999999999")
	if !errors.Is(err, domainErrors.ErrUserNotFound) ||
		!errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("GetSecurityState(missing) error = %v, want ErrUserNotFound and sql.ErrNoRows", err)
	}

	_, err = repo.GetIdentityState(ctx, "49999999-9999-4999-8999-999999999999")
	if !errors.Is(err, domainErrors.ErrUserNotFound) ||
		!errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("GetIdentityState(missing) error = %v, want ErrUserNotFound and sql.ErrNoRows", err)
	}
}

func TestRepositoryIntegrationCRUD(t *testing.T) {
	repo := newPostgresUserIntegrationRepository(t)
	ctx := context.Background()
	createdAt := time.Date(2026, time.July, 23, 8, 30, 0, 0, time.UTC)
	initialUpdatedAt := time.Date(2026, time.July, 23, 8, 45, 0, 0, time.UTC)
	original := &entity.User{
		ID:                "11111111-1111-4111-8111-111111111111",
		Email:             "original@example.com",
		PasswordDigest:    testPasswordDigest("$argon2id$original-hash"),
		Status:            valueobject.StatusActive,
		Role:              valueobject.RoleClient,
		CredentialVersion: entity.InitialCredentialVersion,
		CreatedAt:         createdAt,
		UpdatedAt:         &initialUpdatedAt,
	}

	if err := repo.Create(ctx, original); err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	exists, err := repo.ExistsByID(ctx, original.ID)
	if err != nil {
		t.Fatalf("ExistsByID() error = %v", err)
	}
	if !exists {
		t.Fatal("ExistsByID() = false, want true")
	}

	byID, err := repo.FindByID(ctx, original.ID)
	if err != nil {
		t.Fatalf("FindByID() error = %v", err)
	}
	assertPersistedUserEqual(t, byID, original)
	if byID.PasswordDigest.IsZero() {
		t.Fatal("FindByID() did not rehydrate the full aggregate password digest")
	}

	detail, err := repo.FindDetailByID(ctx, original.ID)
	if err != nil {
		t.Fatalf("FindDetailByID() error = %v", err)
	}
	if detail.ID != original.ID ||
		detail.Email != original.Email ||
		detail.Role != original.Role ||
		detail.Status != original.Status ||
		!detail.CreatedAt.Equal(original.CreatedAt) {
		t.Fatalf("FindDetailByID() = %#v, want public fields from %#v", detail, original)
	}

	statusSnapshot, err := repo.FindStatusSnapshotByID(ctx, original.ID)
	if err != nil {
		t.Fatalf("FindStatusSnapshotByID() error = %v", err)
	}
	if statusSnapshot.ID != original.ID ||
		statusSnapshot.Role != original.Role ||
		statusSnapshot.Status != original.Status {
		t.Fatalf(
			"FindStatusSnapshotByID() = %#v, want status preconditions from %#v",
			statusSnapshot,
			original,
		)
	}

	byEmail, err := repo.FindByEmail(ctx, original.Email)
	if err != nil {
		t.Fatalf("FindByEmail() error = %v", err)
	}
	assertPersistedUserEqual(t, byEmail, original)

	missingByEmail, err := repo.FindByEmail(ctx, "missing@example.com")
	if err != nil {
		t.Fatalf("FindByEmail(missing) error = %v", err)
	}
	if missingByEmail != nil {
		t.Fatalf("FindByEmail(missing) = %#v, want nil", missingByEmail)
	}

	_, err = repo.FindByID(ctx, "99999999-9999-4999-8999-999999999999")
	if !errors.Is(err, domainErrors.ErrUserNotFound) {
		t.Fatalf("FindByID(missing) error = %v, want ErrUserNotFound", err)
	}
	if !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("FindByID(missing) error = %v, want preserved sql.ErrNoRows", err)
	}

	_, err = repo.FindDetailByID(ctx, "99999999-9999-4999-8999-999999999999")
	if !errors.Is(err, domainErrors.ErrUserNotFound) {
		t.Fatalf("FindDetailByID(missing) error = %v, want ErrUserNotFound", err)
	}
	if !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("FindDetailByID(missing) error = %v, want preserved sql.ErrNoRows", err)
	}

	_, err = repo.FindStatusSnapshotByID(ctx, "99999999-9999-4999-8999-999999999999")
	if !errors.Is(err, domainErrors.ErrUserNotFound) {
		t.Fatalf("FindStatusSnapshotByID(missing) error = %v, want ErrUserNotFound", err)
	}
	if !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("FindStatusSnapshotByID(missing) error = %v, want preserved sql.ErrNoRows", err)
	}

	updateStartedAt := time.Now().UTC()
	update := userupdating.UserUpdateCommand{
		UserID:         original.ID,
		Email:          "updated@example.com",
		PasswordDigest: testPasswordDigest("$argon2id$updated-hash"),
	}
	if err := repo.UpdateUser(ctx, update); err != nil {
		t.Fatalf("UpdateUser() error = %v", err)
	}

	updated, err := repo.FindByID(ctx, original.ID)
	if err != nil {
		t.Fatalf("FindByID(updated) error = %v", err)
	}
	if updated.Email != update.Email || updated.PasswordDigest.Encoded() != update.PasswordDigest.Encoded() ||
		updated.Status != original.Status || updated.Role != original.Role {
		t.Fatalf("updated user = %#v, want values from %#v", updated, update)
	}
	if updated.CredentialVersion != original.CredentialVersion+1 {
		t.Fatalf(
			"credential version after password update = %d, want %d",
			updated.CredentialVersion,
			original.CredentialVersion+1,
		)
	}
	if !updated.CreatedAt.Equal(original.CreatedAt) {
		t.Fatalf("updated created_at = %v, want %v", updated.CreatedAt, original.CreatedAt)
	}
	if updated.UpdatedAt == nil || updated.UpdatedAt.Before(updateStartedAt.Add(-time.Second)) {
		t.Fatalf("updated_at = %v, want timestamp set by Update", updated.UpdatedAt)
	}

	statusResult, err := repo.UpdateStatus(
		ctx,
		original.ID,
		valueobject.StatusActive,
		valueobject.StatusSuspended,
	)
	if err != nil {
		t.Fatalf("UpdateStatus() error = %v", err)
	}
	if statusResult.Outcome != repository.UserStatusUpdateApplied ||
		statusResult.CurrentStatus != valueobject.StatusSuspended {
		t.Fatalf("UpdateStatus() result = %#v, want applied suspended", statusResult)
	}
	updated, err = repo.FindByID(ctx, original.ID)
	if err != nil {
		t.Fatalf("FindByID(status updated) error = %v", err)
	}
	if updated.Status != valueobject.StatusSuspended {
		t.Fatalf("updated status = %s, want suspended", updated.Status)
	}

	if err := repo.UpdateUser(ctx, userupdating.UserUpdateCommand{UserID: original.ID}); !errors.Is(err, domainErrors.ErrBadRequest) {
		t.Fatalf("UpdateUser(no fields) error = %v, want ErrBadRequest", err)
	}
	if err := repo.UpdateEmail(ctx, useremailchange.UserEmailUpdateCommand{
		UserID: "99999999-9999-4999-8999-999999999999",
		Email:  "missing@example.com",
	}); !errors.Is(err, domainErrors.ErrUserNotFound) {
		t.Fatalf("UpdateEmail(missing) error = %v, want ErrUserNotFound", err)
	}

	if err := repo.Delete(ctx, original.ID); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}
	exists, err = repo.ExistsByID(ctx, original.ID)
	if err != nil {
		t.Fatalf("ExistsByID(after delete) error = %v", err)
	}
	if exists {
		t.Fatal("ExistsByID(after delete) = true, want false")
	}
	if err := repo.Delete(ctx, original.ID); !errors.Is(err, domainErrors.ErrUserNotFound) {
		t.Fatalf("Delete(missing) error = %v, want ErrUserNotFound", err)
	}
}

func TestRepositoryIntegrationGenericUserUpdateCommand(t *testing.T) {
	repo := newPostgresUserIntegrationRepository(t)
	ctx := context.Background()
	createdAt := time.Date(2026, time.July, 27, 6, 0, 0, 0, time.UTC)
	original := integrationUser(
		"21111111-1111-4111-8111-111111111111",
		"generic-update@example.com",
		valueobject.RoleClient,
		valueobject.StatusActive,
		createdAt,
	)
	conflicting := integrationUser(
		"22222222-2222-4222-8222-222222222222",
		"generic-conflict@example.com",
		valueobject.RoleClient,
		valueobject.StatusActive,
		createdAt,
	)
	if err := repo.Create(ctx, original); err != nil {
		t.Fatalf("Create(original) error = %v", err)
	}
	if err := repo.Create(ctx, conflicting); err != nil {
		t.Fatalf("Create(conflicting) error = %v", err)
	}

	if err := repo.UpdateUser(ctx, userupdating.UserUpdateCommand{
		UserID: original.ID,
		Email:  "email-only@example.com",
	}); err != nil {
		t.Fatalf("UpdateUser(email only) error = %v", err)
	}
	afterEmail, err := repo.FindByID(ctx, original.ID)
	if err != nil {
		t.Fatalf("FindByID(after email) error = %v", err)
	}
	if afterEmail.Email != "email-only@example.com" ||
		afterEmail.PasswordDigest.Encoded() != original.PasswordDigest.Encoded() ||
		afterEmail.Role != original.Role ||
		afterEmail.CredentialVersion != original.CredentialVersion {
		t.Fatalf("email-only update = %#v, want credentials and role unchanged", afterEmail)
	}

	passwordOnlyDigest := testPasswordDigest("$argon2id$password-only-hash")
	if err := repo.UpdateUser(ctx, userupdating.UserUpdateCommand{
		UserID:         original.ID,
		PasswordDigest: passwordOnlyDigest,
	}); err != nil {
		t.Fatalf("UpdateUser(password only) error = %v", err)
	}
	afterPassword, err := repo.FindByID(ctx, original.ID)
	if err != nil {
		t.Fatalf("FindByID(after password) error = %v", err)
	}
	if afterPassword.Email != afterEmail.Email ||
		afterPassword.PasswordDigest.Encoded() != passwordOnlyDigest.Encoded() ||
		afterPassword.Role != original.Role ||
		afterPassword.CredentialVersion != original.CredentialVersion+1 {
		t.Fatalf("password-only update = %#v, want email and role unchanged", afterPassword)
	}

	mixedDigest := testPasswordDigest("$argon2id$mixed-update-hash")
	command := userupdating.UserUpdateCommand{
		UserID:         original.ID,
		Email:          "generic-updated@example.com",
		PasswordDigest: mixedDigest,
	}
	if err := repo.UpdateUser(ctx, command); err != nil {
		t.Fatalf("UpdateUser(mixed) error = %v", err)
	}
	stored, err := repo.FindByID(ctx, original.ID)
	if err != nil {
		t.Fatalf("FindByID(after mixed update) error = %v", err)
	}
	if stored.Email != command.Email ||
		stored.PasswordDigest.Encoded() != mixedDigest.Encoded() ||
		stored.Role != original.Role ||
		stored.Status != original.Status ||
		stored.CredentialVersion != original.CredentialVersion+2 {
		t.Fatalf("mixed update = %#v, want email/password atomically updated", stored)
	}

	conflictDigest := testPasswordDigest("$argon2id$conflict-must-not-persist")
	assertEmailConflictError(t, repo.UpdateUser(ctx, userupdating.UserUpdateCommand{
		UserID:         original.ID,
		Email:          conflicting.Email,
		PasswordDigest: conflictDigest,
	}))
	afterConflict, err := repo.FindByID(ctx, original.ID)
	if err != nil {
		t.Fatalf("FindByID(after conflict) error = %v", err)
	}
	if afterConflict.Email != stored.Email ||
		afterConflict.PasswordDigest.Encoded() != stored.PasswordDigest.Encoded() ||
		afterConflict.CredentialVersion != stored.CredentialVersion {
		t.Fatalf("failed mixed update partially persisted: before=%#v after=%#v", stored, afterConflict)
	}

	err = repo.UpdateUser(ctx, userupdating.UserUpdateCommand{
		UserID: "29999999-9999-4999-8999-999999999999",
		Email:  "missing@example.com",
	})
	if !errors.Is(err, domainErrors.ErrUserNotFound) {
		t.Fatalf("UpdateUser(missing) error = %v, want ErrUserNotFound", err)
	}

	err = repo.UpdateUser(ctx, userupdating.UserUpdateCommand{UserID: original.ID})
	if !errors.Is(err, domainErrors.ErrBadRequest) {
		t.Fatalf("UpdateUser(no fields) error = %v, want ErrBadRequest", err)
	}
}

func TestRepositoryIntegrationFocusedEmailUpdateCommand(t *testing.T) {
	repo := newPostgresUserIntegrationRepository(t)
	ctx := context.Background()
	createdAt := time.Date(2026, time.July, 28, 8, 0, 0, 0, time.UTC)
	user := integrationUser(
		"23111111-1111-4111-8111-111111111111",
		"email-command@example.com",
		valueobject.RoleOperator,
		valueobject.StatusActive,
		createdAt,
	)
	conflicting := integrationUser(
		"23222222-2222-4222-8222-222222222222",
		"email-command-conflict@example.com",
		valueobject.RoleClient,
		valueobject.StatusInactive,
		createdAt,
	)
	if err := repo.Create(ctx, user); err != nil {
		t.Fatalf("Create(user) error = %v", err)
	}
	if err := repo.Create(ctx, conflicting); err != nil {
		t.Fatalf("Create(conflicting) error = %v", err)
	}

	updateStartedAt := time.Now().UTC()
	const updatedEmail = "email-command-updated@example.com"
	if err := repo.UpdateEmail(ctx, useremailchange.UserEmailUpdateCommand{
		UserID: user.ID,
		Email:  updatedEmail,
	}); err != nil {
		t.Fatalf("UpdateEmail() error = %v", err)
	}

	updated, err := repo.FindByID(ctx, user.ID)
	if err != nil {
		t.Fatalf("FindByID(updated) error = %v", err)
	}
	if updated.Email != updatedEmail {
		t.Fatalf("updated email = %q, want %q", updated.Email, updatedEmail)
	}
	if updated.PasswordDigest.Encoded() != user.PasswordDigest.Encoded() ||
		updated.Role != user.Role ||
		updated.Status != user.Status ||
		updated.CredentialVersion != user.CredentialVersion ||
		!updated.CreatedAt.Equal(user.CreatedAt) {
		t.Fatalf("focused email update changed unrelated state: %#v", updated)
	}
	if updated.UpdatedAt == nil || updated.UpdatedAt.Before(updateStartedAt.Add(-time.Second)) {
		t.Fatalf("updated_at = %v, want email-update timestamp", updated.UpdatedAt)
	}

	assertEmailConflictError(t, repo.UpdateEmail(ctx, useremailchange.UserEmailUpdateCommand{
		UserID: user.ID,
		Email:  conflicting.Email,
	}))
	afterConflict, err := repo.FindByID(ctx, user.ID)
	if err != nil {
		t.Fatalf("FindByID(after conflict) error = %v", err)
	}
	if afterConflict.Email != updatedEmail {
		t.Fatalf("email after conflict = %q, want %q", afterConflict.Email, updatedEmail)
	}

	if err := repo.UpdateEmail(ctx, useremailchange.UserEmailUpdateCommand{
		UserID: user.ID,
	}); !errors.Is(err, domainErrors.ErrBadRequest) {
		t.Fatalf("UpdateEmail(empty email) error = %v, want ErrBadRequest", err)
	}
	if err := repo.UpdateEmail(ctx, useremailchange.UserEmailUpdateCommand{
		UserID: "99999999-9999-4999-8999-999999999999",
		Email:  "email-command-missing@example.com",
	}); !errors.Is(err, domainErrors.ErrUserNotFound) {
		t.Fatalf("UpdateEmail(missing user) error = %v, want ErrUserNotFound", err)
	}

	cancelledCtx, cancel := context.WithCancel(ctx)
	cancel()
	err = repo.UpdateEmail(cancelledCtx, useremailchange.UserEmailUpdateCommand{
		UserID: user.ID,
		Email:  "email-command-cancelled@example.com",
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("UpdateEmail(cancelled) error = %v, want context.Canceled", err)
	}
}

func TestRepositoryIntegrationFocusedRoleUpdateCommand(t *testing.T) {
	repo := newPostgresUserIntegrationRepository(t)
	ctx := context.Background()
	createdAt := time.Date(2026, time.July, 27, 7, 0, 0, 0, time.UTC)
	original := integrationUser(
		"31111111-1111-4111-8111-111111111111",
		"role-command@example.com",
		valueobject.RoleClient,
		valueobject.StatusSuspended,
		createdAt,
	)
	if err := repo.Create(ctx, original); err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	command := userrolechange.UserRoleUpdateCommand{
		UserID:        original.ID,
		ExpectedRole:  valueobject.RoleClient,
		RequestedRole: valueobject.RoleOperator,
	}
	result, err := repo.UpdateRole(ctx, command)
	if err != nil {
		t.Fatalf("UpdateRole() error = %v", err)
	}
	assertRoleUpdateResult(
		t,
		result,
		userrolechange.UserRoleUpdateApplied,
		valueobject.RoleOperator,
	)

	stored, err := repo.FindByID(ctx, original.ID)
	if err != nil {
		t.Fatalf("FindByID(updated) error = %v", err)
	}
	if stored.Role != command.RequestedRole {
		t.Fatalf("stored role = %s, want %s", stored.Role, command.RequestedRole)
	}
	if stored.Email != original.Email ||
		stored.PasswordDigest != original.PasswordDigest ||
		stored.Status != original.Status ||
		stored.CredentialVersion != original.CredentialVersion ||
		!stored.CreatedAt.Equal(original.CreatedAt) {
		t.Fatalf("role update changed fields outside its command: got %#v, original %#v", stored, original)
	}
	if stored.UpdatedAt == nil {
		t.Fatal("stored updated_at = nil, want role-update timestamp")
	}

	oldUpdatedAt := time.Date(2000, time.January, 1, 0, 0, 0, 0, time.UTC)
	if _, err := repo.db.ExecContext(
		ctx,
		"UPDATE users SET updated_at = $1 WHERE id = $2",
		oldUpdatedAt,
		original.ID,
	); err != nil {
		t.Fatalf("set deterministic prior updated_at: %v", err)
	}
	sameRoleCommand := userrolechange.UserRoleUpdateCommand{
		UserID:        original.ID,
		ExpectedRole:  valueobject.RoleOperator,
		RequestedRole: valueobject.RoleOperator,
	}
	result, err = repo.UpdateRole(ctx, sameRoleCommand)
	if err != nil {
		t.Fatalf("UpdateRole(same role) error = %v", err)
	}
	assertRoleUpdateResult(
		t,
		result,
		userrolechange.UserRoleUpdateApplied,
		valueobject.RoleOperator,
	)
	stored, err = repo.FindByID(ctx, original.ID)
	if err != nil {
		t.Fatalf("FindByID(same role) error = %v", err)
	}
	if stored.UpdatedAt == nil || !stored.UpdatedAt.After(oldUpdatedAt) {
		t.Fatalf("same-role updated_at = %v, want after %v", stored.UpdatedAt, oldUpdatedAt)
	}
	if stored.CredentialVersion != original.CredentialVersion {
		t.Fatalf(
			"credential version after repeated role update = %d, want %d",
			stored.CredentialVersion,
			original.CredentialVersion,
		)
	}

	alreadyApplied, err := repo.UpdateRole(ctx, command)
	if err != nil {
		t.Fatalf("UpdateRole(already applied) error = %v", err)
	}
	assertRoleUpdateResult(
		t,
		alreadyApplied,
		userrolechange.UserRoleUpdateAlreadyApplied,
		valueobject.RoleOperator,
	)

	conflict, err := repo.UpdateRole(ctx, userrolechange.UserRoleUpdateCommand{
		UserID:        original.ID,
		ExpectedRole:  valueobject.RoleClient,
		RequestedRole: valueobject.RoleAdmin,
	})
	if err != nil {
		t.Fatalf("UpdateRole(conflict) error = %v", err)
	}
	assertRoleUpdateResult(
		t,
		conflict,
		userrolechange.UserRoleUpdateConflict,
		valueobject.RoleOperator,
	)
	stored, err = repo.FindByID(ctx, original.ID)
	if err != nil {
		t.Fatalf("FindByID(after conflicting update) error = %v", err)
	}
	if stored.Role != valueobject.RoleOperator {
		t.Fatalf("role after conflicting update = %s, want operator unchanged", stored.Role)
	}

	notFound, err := repo.UpdateRole(ctx, userrolechange.UserRoleUpdateCommand{
		UserID:        "39999999-9999-4999-8999-999999999999",
		ExpectedRole:  valueobject.RoleClient,
		RequestedRole: valueobject.RoleOperator,
	})
	if err != nil {
		t.Fatalf("UpdateRole(missing) error = %v", err)
	}
	assertRoleUpdateResult(
		t,
		notFound,
		userrolechange.UserRoleUpdateNotFound,
		valueobject.RoleUnknown,
	)

	for _, invalid := range []userrolechange.UserRoleUpdateCommand{
		{
			UserID:        original.ID,
			ExpectedRole:  valueobject.RoleUnknown,
			RequestedRole: valueobject.RoleClient,
		},
		{
			UserID:        original.ID,
			ExpectedRole:  valueobject.RoleOperator,
			RequestedRole: valueobject.RoleUnknown,
		},
	} {
		if _, err := repo.UpdateRole(ctx, invalid); !errors.Is(err, domainErrors.ErrBadRequest) {
			t.Fatalf("UpdateRole(invalid role) error = %v, want ErrBadRequest", err)
		}
	}

	cancelledCtx, cancel := context.WithCancel(ctx)
	cancel()
	if _, err := repo.UpdateRole(cancelledCtx, sameRoleCommand); !errors.Is(err, context.Canceled) {
		t.Fatalf("UpdateRole(cancelled) error = %v, want context.Canceled", err)
	}

	if _, err := repo.db.ExecContext(
		ctx,
		"UPDATE users SET role = 99 WHERE id = $1",
		original.ID,
	); err != nil {
		t.Fatalf("seed unknown authoritative role: %v", err)
	}
	if _, err := repo.UpdateRole(ctx, userrolechange.UserRoleUpdateCommand{
		UserID:        original.ID,
		ExpectedRole:  valueobject.RoleClient,
		RequestedRole: valueobject.RoleAdmin,
	}); err == nil {
		t.Fatal("UpdateRole(unknown authoritative role) error = nil")
	}
}

func TestRepositoryIntegrationConcurrentConflictingRoleUpdates(t *testing.T) {
	repo := newConcurrentPostgresUserIntegrationRepository(t)
	ctx := context.Background()
	user := integrationUser(
		"32222222-2222-4222-8222-222222222222",
		"concurrent-role-command@example.com",
		valueobject.RoleClient,
		valueobject.StatusActive,
		time.Now().UTC(),
	)
	if err := repo.Create(ctx, user); err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	commands := []userrolechange.UserRoleUpdateCommand{
		{
			UserID:        user.ID,
			ExpectedRole:  valueobject.RoleClient,
			RequestedRole: valueobject.RoleOperator,
		},
		{
			UserID:        user.ID,
			ExpectedRole:  valueobject.RoleClient,
			RequestedRole: valueobject.RoleAdmin,
		},
	}
	ready := &sync.WaitGroup{}
	ready.Add(len(commands))
	start := make(chan struct{})
	results := make([]userrolechange.UserRoleUpdateResult, len(commands))
	errs := make([]error, len(commands))
	wait := &sync.WaitGroup{}
	wait.Add(len(commands))
	for index := range commands {
		index := index
		go func() {
			defer wait.Done()
			ready.Done()
			<-start
			results[index], errs[index] = repo.UpdateRole(ctx, commands[index])
		}()
	}
	ready.Wait()
	close(start)
	wait.Wait()

	outcomes := map[userrolechange.UserRoleUpdateOutcome]int{}
	for index, err := range errs {
		if err != nil {
			t.Fatalf("UpdateRole(%d) error = %v", index, err)
		}
		outcomes[results[index].Outcome]++
	}
	if outcomes[userrolechange.UserRoleUpdateApplied] != 1 ||
		outcomes[userrolechange.UserRoleUpdateConflict] != 1 {
		t.Fatalf(
			"concurrent outcomes = %v, want one applied and one conflict",
			outcomes,
		)
	}

	stored, err := repo.FindByID(ctx, user.ID)
	if err != nil {
		t.Fatalf("FindByID(after concurrent role updates) error = %v", err)
	}
	if stored.Role != valueobject.RoleOperator && stored.Role != valueobject.RoleAdmin {
		t.Fatalf("stored role = %s, want committed operator or admin role", stored.Role)
	}
	if stored.CredentialVersion != user.CredentialVersion {
		t.Fatalf(
			"credential version = %d, want unchanged %d",
			stored.CredentialVersion,
			user.CredentialVersion,
		)
	}
}

func TestRepositoryIntegrationUpdateStatusCompareAndSetOutcomes(t *testing.T) {
	repo := newPostgresUserIntegrationRepository(t)
	ctx := context.Background()
	user := integrationUser(
		"50000000-0000-4000-8000-000000000001",
		"status-cas@example.com",
		valueobject.RoleClient,
		valueobject.StatusInactive,
		time.Now().UTC(),
	)
	if err := repo.Create(ctx, user); err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	applied, err := repo.UpdateStatus(
		ctx,
		user.ID,
		valueobject.StatusInactive,
		valueobject.StatusActive,
	)
	if err != nil {
		t.Fatalf("UpdateStatus(applied) error = %v", err)
	}
	assertStatusUpdateResult(
		t,
		applied,
		repository.UserStatusUpdateApplied,
		valueobject.StatusActive,
	)

	alreadyApplied, err := repo.UpdateStatus(
		ctx,
		user.ID,
		valueobject.StatusInactive,
		valueobject.StatusActive,
	)
	if err != nil {
		t.Fatalf("UpdateStatus(already applied) error = %v", err)
	}
	assertStatusUpdateResult(
		t,
		alreadyApplied,
		repository.UserStatusUpdateAlreadyApplied,
		valueobject.StatusActive,
	)

	conflict, err := repo.UpdateStatus(
		ctx,
		user.ID,
		valueobject.StatusSuspended,
		valueobject.StatusInactive,
	)
	if err != nil {
		t.Fatalf("UpdateStatus(conflict) error = %v", err)
	}
	assertStatusUpdateResult(
		t,
		conflict,
		repository.UserStatusUpdateConflict,
		valueobject.StatusActive,
	)

	notFound, err := repo.UpdateStatus(
		ctx,
		"59999999-9999-4999-8999-999999999999",
		valueobject.StatusActive,
		valueobject.StatusSuspended,
	)
	if err != nil {
		t.Fatalf("UpdateStatus(not found) error = %v", err)
	}
	assertStatusUpdateResult(
		t,
		notFound,
		repository.UserStatusUpdateNotFound,
		valueobject.StatusUnknown,
	)

	if _, err := repo.UpdateStatus(
		ctx,
		user.ID,
		valueobject.StatusUnknown,
		valueobject.StatusActive,
	); err == nil {
		t.Fatal("UpdateStatus(unknown expected) error = nil")
	}
	if _, err := repo.UpdateStatus(
		ctx,
		user.ID,
		valueobject.StatusActive,
		valueobject.StatusUnknown,
	); err == nil {
		t.Fatal("UpdateStatus(unknown requested) error = nil")
	}
	if _, err := repo.UpdateStatus(
		ctx,
		user.ID,
		valueobject.StatusActive,
		valueobject.StatusActive,
	); err == nil {
		t.Fatal("UpdateStatus(equal statuses) error = nil")
	}

	cancelledCtx, cancel := context.WithCancel(ctx)
	cancel()
	_, err = repo.UpdateStatus(
		cancelledCtx,
		user.ID,
		valueobject.StatusActive,
		valueobject.StatusSuspended,
	)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf(
			"UpdateStatus(cancelled) error = %v, want preserved context.Canceled",
			err,
		)
	}

	stored, err := repo.FindByID(ctx, user.ID)
	if err != nil {
		t.Fatalf("FindByID(after generic status update) error = %v", err)
	}
	if stored.Status != valueobject.StatusActive {
		t.Fatalf(
			"status after generic Update = %s, want active unchanged",
			stored.Status,
		)
	}

	if _, err := repo.db.ExecContext(
		ctx,
		"UPDATE users SET status = 99 WHERE id = $1",
		user.ID,
	); err != nil {
		t.Fatalf("seed unknown authoritative status: %v", err)
	}
	if _, err := repo.UpdateStatus(
		ctx,
		user.ID,
		valueobject.StatusActive,
		valueobject.StatusSuspended,
	); err == nil {
		t.Fatal("UpdateStatus(unknown authoritative status) error = nil")
	}
}

func TestRepositoryIntegrationConcurrentIdenticalStatusUpdates(t *testing.T) {
	tests := []struct {
		name      string
		initial   valueobject.UserStatus
		requested valueobject.UserStatus
	}{
		{
			name:      "activation",
			initial:   valueobject.StatusInactive,
			requested: valueobject.StatusActive,
		},
		{
			name:      "suspension",
			initial:   valueobject.StatusActive,
			requested: valueobject.StatusSuspended,
		},
		{
			name:      "reactivation",
			initial:   valueobject.StatusSuspended,
			requested: valueobject.StatusActive,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			repo := newPostgresUserIntegrationRepository(t)
			ctx := context.Background()
			user := integrationUser(
				"60000000-0000-4000-8000-000000000001",
				"concurrent-"+test.name+"@example.com",
				valueobject.RoleClient,
				test.initial,
				time.Now().UTC(),
			)
			if err := repo.Create(ctx, user); err != nil {
				t.Fatalf("Create() error = %v", err)
			}

			start := make(chan struct{})
			var wait sync.WaitGroup
			wait.Add(2)
			results := make([]repository.UserStatusUpdateResult, 2)
			errs := make([]error, 2)
			for index := range results {
				index := index
				go func() {
					defer wait.Done()
					<-start
					results[index], errs[index] = repo.UpdateStatus(
						ctx,
						user.ID,
						test.initial,
						test.requested,
					)
				}()
			}
			close(start)
			wait.Wait()

			outcomes := map[repository.UserStatusUpdateOutcome]int{}
			for index, err := range errs {
				if err != nil {
					t.Fatalf("UpdateStatus(%d) error = %v", index, err)
				}
				outcomes[results[index].Outcome]++
			}
			if outcomes[repository.UserStatusUpdateApplied] != 1 ||
				outcomes[repository.UserStatusUpdateAlreadyApplied] != 1 {
				t.Fatalf(
					"concurrent outcomes = %v, want one applied and one already applied",
					outcomes,
				)
			}

			stored, err := repo.FindByID(ctx, user.ID)
			if err != nil {
				t.Fatalf("FindByID(after concurrent updates) error = %v", err)
			}
			if stored.Status != test.requested {
				t.Fatalf(
					"stored status = %s, want %s",
					stored.Status,
					test.requested,
				)
			}
		})
	}
}

func TestRepositoryIntegrationTranslatesEmailUniqueViolations(t *testing.T) {
	repo := newPostgresUserIntegrationRepository(t)
	ctx := context.Background()
	createdAt := time.Date(2026, time.July, 23, 13, 0, 0, 0, time.UTC)
	first := integrationUser(
		"10000000-0000-4000-8000-000000000001",
		"first@example.com",
		valueobject.RoleClient,
		valueobject.StatusActive,
		createdAt,
	)
	second := integrationUser(
		"10000000-0000-4000-8000-000000000002",
		"second@example.com",
		valueobject.RoleClient,
		valueobject.StatusActive,
		createdAt,
	)

	if err := repo.Create(ctx, first); err != nil {
		t.Fatalf("Create(first) error = %v", err)
	}

	duplicateEmail := integrationUser(
		"10000000-0000-4000-8000-000000000003",
		first.Email,
		valueobject.RoleClient,
		valueobject.StatusActive,
		createdAt,
	)
	assertEmailConflictError(t, repo.Create(ctx, duplicateEmail))

	duplicateID := integrationUser(
		first.ID,
		"different@example.com",
		valueobject.RoleClient,
		valueobject.StatusActive,
		createdAt,
	)
	primaryKeyErr := repo.Create(ctx, duplicateID)
	if primaryKeyErr == nil {
		t.Fatal("Create(duplicate ID) error = nil, want users_pkey violation")
	}
	if errors.Is(primaryKeyErr, domainErrors.ErrEmailAlreadyExists) {
		t.Fatalf("Create(duplicate ID) error = %v, must not be classified as email conflict", primaryKeyErr)
	}
	assertPostgresConstraint(t, primaryKeyErr, "23505", "users_pkey")

	if err := repo.Create(ctx, second); err != nil {
		t.Fatalf("Create(second) error = %v", err)
	}
	assertEmailConflictError(t, repo.UpdateEmail(ctx, useremailchange.UserEmailUpdateCommand{
		UserID: second.ID,
		Email:  first.Email,
	}))

	storedSecond, err := repo.FindByID(ctx, second.ID)
	if err != nil {
		t.Fatalf("FindByID(second after conflict) error = %v", err)
	}
	if storedSecond.Email != second.Email {
		t.Fatalf("email after failed update = %q, want unchanged %q", storedSecond.Email, second.Email)
	}

	const uniqueEmail = "second-updated@example.com"
	if err := repo.UpdateEmail(ctx, useremailchange.UserEmailUpdateCommand{UserID: second.ID, Email: uniqueEmail}); err != nil {
		t.Fatalf("UpdateEmail(unique email) error = %v", err)
	}
	storedSecond, err = repo.FindByID(ctx, second.ID)
	if err != nil {
		t.Fatalf("FindByID(second after successful update) error = %v", err)
	}
	if storedSecond.Email != uniqueEmail {
		t.Fatalf("email after successful update = %q, want %q", storedSecond.Email, uniqueEmail)
	}
}

func TestRepositoryIntegrationUpdatesPasswordAndCredentialVersionAtomically(t *testing.T) {
	repo := newPostgresUserIntegrationRepository(t)
	ctx := context.Background()
	user := integrationUser(
		"30000000-0000-4000-8000-000000000001",
		"credentials@example.com",
		valueobject.RoleClient,
		valueobject.StatusActive,
		time.Now().UTC(),
	)
	if err := repo.Create(ctx, user); err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	initialVersion, err := repo.GetCredentialVersion(ctx, user.ID)
	if err != nil {
		t.Fatalf("GetCredentialVersion(initial) error = %v", err)
	}
	if initialVersion != entity.InitialCredentialVersion {
		t.Fatalf("initial credential version = %d, want %d", initialVersion, entity.InitialCredentialVersion)
	}

	updatedHash := testPasswordDigest("$argon2id$updated-credential-hash")
	updatedVersion, err := repo.UpdatePassword(ctx, user.ID, updatedHash)
	if err != nil {
		t.Fatalf("UpdatePassword() error = %v", err)
	}
	if updatedVersion != initialVersion+1 {
		t.Fatalf("updated credential version = %d, want %d", updatedVersion, initialVersion+1)
	}

	updated, err := repo.FindByID(ctx, user.ID)
	if err != nil {
		t.Fatalf("FindByID(after update) error = %v", err)
	}
	if updated.PasswordDigest.Encoded() != updatedHash.Encoded() {
		t.Fatalf("updated password hash = %q, want %q", updated.PasswordDigest.Encoded(), updatedHash.Encoded())
	}
	if updated.CredentialVersion != updatedVersion {
		t.Fatalf("stored credential version = %d, want %d", updated.CredentialVersion, updatedVersion)
	}

	_, err = repo.UpdatePassword(
		ctx,
		user.ID,
		testPasswordDigest("$reject-password-update$"),
	)
	if err == nil {
		t.Fatal("UpdatePassword(rejected hash) error = nil, want PostgreSQL constraint failure")
	}
	if errors.Is(err, domainErrors.ErrEmailAlreadyExists) {
		t.Fatalf("password constraint failure was misclassified as email conflict: %v", err)
	}

	afterRollback, err := repo.FindByID(ctx, user.ID)
	if err != nil {
		t.Fatalf("FindByID(after rollback) error = %v", err)
	}
	if afterRollback.PasswordDigest.Encoded() != updatedHash.Encoded() {
		t.Fatalf(
			"password after failed statement = %q, want unchanged %q",
			afterRollback.PasswordDigest.Encoded(),
			updatedHash.Encoded(),
		)
	}
	if afterRollback.CredentialVersion != updatedVersion {
		t.Fatalf(
			"credential version after failed statement = %d, want unchanged %d",
			afterRollback.CredentialVersion,
			updatedVersion,
		)
	}

	_, err = repo.UpdatePassword(
		ctx,
		"39999999-9999-4999-8999-999999999999",
		updatedHash,
	)
	if !errors.Is(err, domainErrors.ErrUserNotFound) || !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("UpdatePassword(missing) error = %v, want ErrUserNotFound and sql.ErrNoRows", err)
	}
}

func TestRepositoryIntegrationListFiltersCountsAndPaginatesDeterministically(t *testing.T) {
	repo := newPostgresUserIntegrationRepository(t)
	ctx := context.Background()
	createdAt := time.Date(2026, time.July, 23, 12, 0, 0, 0, time.UTC)
	users := []*entity.User{
		integrationUser("00000000-0000-4000-8000-000000000001", "first@example.com", valueobject.RoleClient, valueobject.StatusActive, createdAt),
		integrationUser("00000000-0000-4000-8000-000000000002", "second@example.com", valueobject.RoleClient, valueobject.StatusActive, createdAt),
		integrationUser("00000000-0000-4000-8000-000000000003", "third@example.com", valueobject.RoleClient, valueobject.StatusActive, createdAt),
		integrationUser("00000000-0000-4000-8000-000000000004", "operator@example.com", valueobject.RoleOperator, valueobject.StatusActive, createdAt),
		integrationUser("00000000-0000-4000-8000-000000000005", "suspended@example.com", valueobject.RoleClient, valueobject.StatusSuspended, createdAt),
		integrationUser("00000000-0000-4000-8000-000000000006", "outside@other.test", valueobject.RoleClient, valueobject.StatusActive, createdAt),
	}
	for _, user := range users {
		if err := repo.Create(ctx, user); err != nil {
			t.Fatalf("Create(%s) error = %v", user.ID, err)
		}
	}

	criteria := userlisting.UserListCriteria{
		Roles:    []valueobject.UserRole{valueobject.RoleClient},
		Statuses: []valueobject.UserStatus{valueobject.StatusActive},
		Search:   "example.com",
	}
	firstPage, err := repo.ListUsers(ctx, 0, 2, criteria)
	if err != nil {
		t.Fatalf("ListUsers(first page) error = %v", err)
	}
	if firstPage.Total != 3 {
		t.Fatalf("ListUsers(first page) total = %d, want 3", firstPage.Total)
	}
	assertUserListIDs(t, firstPage.Items, []string{
		"00000000-0000-4000-8000-000000000003",
		"00000000-0000-4000-8000-000000000002",
	})

	secondPage, err := repo.ListUsers(ctx, 2, 2, criteria)
	if err != nil {
		t.Fatalf("ListUsers(second page) error = %v", err)
	}
	if secondPage.Total != 3 {
		t.Fatalf("ListUsers(second page) total = %d, want 3", secondPage.Total)
	}
	assertUserListIDs(t, secondPage.Items, []string{"00000000-0000-4000-8000-000000000001"})

	repeatedPage, err := repo.ListUsers(ctx, 0, 2, criteria)
	if err != nil {
		t.Fatalf("ListUsers(repeated page) error = %v", err)
	}
	if repeatedPage.Total != firstPage.Total {
		t.Fatalf("ListUsers(repeated page) total = %d, want %d", repeatedPage.Total, firstPage.Total)
	}
	assertUserListIDs(t, repeatedPage.Items, []string{
		"00000000-0000-4000-8000-000000000003",
		"00000000-0000-4000-8000-000000000002",
	})

	empty, err := repo.ListUsers(ctx, 0, 10, userlisting.UserListCriteria{Search: "not-present"})
	if err != nil {
		t.Fatalf("ListUsers(empty) error = %v", err)
	}
	if len(empty.Items) != 0 || empty.Total != 0 {
		t.Fatalf("ListUsers(empty) = %#v, want empty items and total 0", empty)
	}
}

func newPostgresUserIntegrationRepository(t *testing.T) *Repository {
	t.Helper()
	dsn := os.Getenv("GOAT_POSTGRES_TEST_DSN")
	if dsn == "" {
		t.Skip("set GOAT_POSTGRES_TEST_DSN to run PostgreSQL integration tests")
	}

	db, err := sqlx.Connect("pgx", dsn)
	if err != nil {
		t.Fatalf("connect to PostgreSQL: %v", err)
	}
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	t.Cleanup(func() {
		if err := db.Close(); err != nil {
			t.Errorf("close PostgreSQL: %v", err)
		}
	})

	const schema = `
		CREATE TEMP TABLE users (
			id UUID PRIMARY KEY,
			email TEXT NOT NULL UNIQUE,
			password TEXT NOT NULL,
			status SMALLINT NOT NULL,
			role SMALLINT NOT NULL,
			credential_version BIGINT NOT NULL DEFAULT 1 CHECK (credential_version > 0),
			created_at TIMESTAMP NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMP NULL,
			CONSTRAINT reject_password_update CHECK (password <> '$reject-password-update$')
		) ON COMMIT PRESERVE ROWS;
		CREATE TEMP TABLE permissions (
			id UUID PRIMARY KEY,
			role SMALLINT NOT NULL,
			action TEXT NOT NULL,
			created_at TIMESTAMP NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMP NULL,
			CONSTRAINT unique_role_action UNIQUE(role, action)
		) ON COMMIT PRESERVE ROWS
	`
	if _, err := db.Exec(schema); err != nil {
		t.Fatalf("create temporary users table: %v", err)
	}

	return NewRepository(db)
}

func newConcurrentPostgresUserIntegrationRepository(t *testing.T) *Repository {
	t.Helper()
	dsn := os.Getenv("GOAT_POSTGRES_TEST_DSN")
	if dsn == "" {
		t.Skip("set GOAT_POSTGRES_TEST_DSN to run PostgreSQL integration tests")
	}

	adminDB, err := sqlx.Connect("pgx", dsn)
	if err != nil {
		t.Fatalf("connect to PostgreSQL: %v", err)
	}
	t.Cleanup(func() {
		if err := adminDB.Close(); err != nil {
			t.Errorf("close PostgreSQL schema administrator: %v", err)
		}
	})

	schemaName := "role_cas_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	quotedSchema := pgx.Identifier{schemaName}.Sanitize()
	if _, err := adminDB.Exec("CREATE SCHEMA " + quotedSchema); err != nil {
		t.Fatalf("create role CAS test schema: %v", err)
	}
	t.Cleanup(func() {
		if _, err := adminDB.Exec("DROP SCHEMA " + quotedSchema + " CASCADE"); err != nil {
			t.Errorf("drop role CAS test schema: %v", err)
		}
	})

	connectionConfig, err := pgx.ParseConfig(dsn)
	if err != nil {
		t.Fatalf("parse PostgreSQL test configuration: %v", err)
	}
	connectionConfig.RuntimeParams["search_path"] = schemaName
	database := stdlib.OpenDB(*connectionConfig)
	database.SetMaxOpenConns(4)
	database.SetMaxIdleConns(4)
	db := sqlx.NewDb(database, "pgx")
	t.Cleanup(func() {
		if err := db.Close(); err != nil {
			t.Errorf("close concurrent PostgreSQL test database: %v", err)
		}
	})
	if err := db.PingContext(context.Background()); err != nil {
		t.Fatalf("ping concurrent PostgreSQL test database: %v", err)
	}

	const schema = `
		CREATE TABLE users (
			id UUID PRIMARY KEY,
			email TEXT NOT NULL UNIQUE,
			password TEXT NOT NULL,
			status SMALLINT NOT NULL,
			role SMALLINT NOT NULL,
			credential_version BIGINT NOT NULL DEFAULT 1 CHECK (credential_version > 0),
			created_at TIMESTAMP NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMP NULL
		)
	`
	if _, err := db.Exec(schema); err != nil {
		t.Fatalf("create concurrent role CAS users table: %v", err)
	}

	return NewRepository(db)
}

func integrationUser(id, email string, role valueobject.UserRole, status valueobject.UserStatus, createdAt time.Time) *entity.User {
	return &entity.User{
		ID:                id,
		Email:             email,
		PasswordDigest:    testPasswordDigest("$argon2id$integration-hash"),
		Role:              role,
		Status:            status,
		CredentialVersion: entity.InitialCredentialVersion,
		CreatedAt:         createdAt,
	}
}

func assertPersistedUserEqual(t *testing.T, got, want *entity.User) {
	t.Helper()
	if got == nil {
		t.Fatal("persisted user is nil")
	}
	if got.ID != want.ID || got.Email != want.Email || got.PasswordDigest.Encoded() != want.PasswordDigest.Encoded() ||
		got.Status != want.Status || got.Role != want.Role ||
		got.CredentialVersion != want.CredentialVersion ||
		!got.CreatedAt.Equal(want.CreatedAt) {
		t.Fatalf("persisted user = %#v, want %#v", got, want)
	}
	if (got.UpdatedAt == nil) != (want.UpdatedAt == nil) {
		t.Fatalf("persisted updated_at = %v, want %v", got.UpdatedAt, want.UpdatedAt)
	}
	if got.UpdatedAt != nil && !got.UpdatedAt.Equal(*want.UpdatedAt) {
		t.Fatalf("persisted updated_at = %v, want %v", got.UpdatedAt, want.UpdatedAt)
	}
}

func assertUserListIDs(t *testing.T, users []userlisting.UserListItem, want []string) {
	t.Helper()
	if len(users) != len(want) {
		t.Fatalf("user count = %d, want %d", len(users), len(want))
	}
	for index := range want {
		if users[index].ID != want[index] {
			t.Fatalf("users[%d].ID = %q, want %q", index, users[index].ID, want[index])
		}
	}
}

func assertStatusUpdateResult(
	t *testing.T,
	result repository.UserStatusUpdateResult,
	wantOutcome repository.UserStatusUpdateOutcome,
	wantStatus valueobject.UserStatus,
) {
	t.Helper()
	if result.Outcome != wantOutcome || result.CurrentStatus != wantStatus {
		t.Fatalf(
			"status update result = %#v, want outcome %d and status %s",
			result,
			wantOutcome,
			wantStatus,
		)
	}
}

func assertRoleUpdateResult(
	t *testing.T,
	result userrolechange.UserRoleUpdateResult,
	wantOutcome userrolechange.UserRoleUpdateOutcome,
	wantRole valueobject.UserRole,
) {
	t.Helper()
	if result.Outcome != wantOutcome || result.CurrentRole != wantRole {
		t.Fatalf(
			"role update result = %#v, want outcome %d and role %s",
			result,
			wantOutcome,
			wantRole,
		)
	}
}

func assertEmailConflictError(t *testing.T, err error) {
	t.Helper()
	if !errors.Is(err, domainErrors.ErrEmailAlreadyExists) {
		t.Fatalf("error = %v, want ErrEmailAlreadyExists", err)
	}
	assertPostgresConstraint(t, err, "23505", "users_email_key")
}

func assertPostgresConstraint(t *testing.T, err error, wantCode, wantConstraint string) {
	t.Helper()
	var postgresErr *pgconn.PgError
	if !errors.As(err, &postgresErr) {
		t.Fatalf("error = %v, want preserved *pgconn.PgError", err)
	}
	if got := postgresErr.Code; got != wantCode {
		t.Fatalf("PostgreSQL SQLSTATE = %q, want %q", got, wantCode)
	}
	if postgresErr.ConstraintName != wantConstraint {
		t.Fatalf("PostgreSQL constraint = %q, want %q", postgresErr.ConstraintName, wantConstraint)
	}
}
