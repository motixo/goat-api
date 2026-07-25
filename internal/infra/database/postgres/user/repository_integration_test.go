package user

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"reflect"
	"sync"
	"testing"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/lib/pq"
	"github.com/motixo/goat-api/internal/domain/entity"
	domainErrors "github.com/motixo/goat-api/internal/domain/errors"
	"github.com/motixo/goat-api/internal/domain/repository"
	"github.com/motixo/goat-api/internal/domain/valueobject"
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
		Password:          valueobject.PasswordFromHash("$argon2id$original-hash"),
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

	updateStartedAt := time.Now().UTC()
	update := &entity.User{
		ID:       original.ID,
		Email:    "updated@example.com",
		Password: valueobject.PasswordFromHash("$argon2id$updated-hash"),
		Status:   valueobject.StatusSuspended,
		Role:     valueobject.RoleOperator,
	}
	if err := repo.Update(ctx, update); err != nil {
		t.Fatalf("Update() error = %v", err)
	}

	updated, err := repo.FindByID(ctx, original.ID)
	if err != nil {
		t.Fatalf("FindByID(updated) error = %v", err)
	}
	if updated.Email != update.Email || updated.Password.Encoded() != update.Password.Encoded() ||
		updated.Status != original.Status || updated.Role != update.Role {
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

	if err := repo.Update(ctx, &entity.User{ID: original.ID}); !errors.Is(err, domainErrors.ErrBadRequest) {
		t.Fatalf("Update(no fields) error = %v, want ErrBadRequest", err)
	}
	if err := repo.Update(ctx, &entity.User{
		ID:    "99999999-9999-4999-8999-999999999999",
		Email: "missing@example.com",
	}); !errors.Is(err, domainErrors.ErrUserNotFound) {
		t.Fatalf("Update(missing) error = %v, want ErrUserNotFound", err)
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

	if err := repo.Update(ctx, &entity.User{
		ID:     user.ID,
		Status: valueobject.StatusSuspended,
	}); !errors.Is(err, domainErrors.ErrBadRequest) {
		t.Fatalf(
			"generic Update(status only) error = %v, want ErrBadRequest",
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
	assertEmailConflictError(t, repo.Update(ctx, &entity.User{
		ID:    second.ID,
		Email: first.Email,
	}))

	storedSecond, err := repo.FindByID(ctx, second.ID)
	if err != nil {
		t.Fatalf("FindByID(second after conflict) error = %v", err)
	}
	if storedSecond.Email != second.Email {
		t.Fatalf("email after failed update = %q, want unchanged %q", storedSecond.Email, second.Email)
	}

	const uniqueEmail = "second-updated@example.com"
	if err := repo.Update(ctx, &entity.User{ID: second.ID, Email: uniqueEmail}); err != nil {
		t.Fatalf("Update(unique email) error = %v", err)
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

	updatedHash := valueobject.PasswordFromHash("$argon2id$updated-credential-hash")
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
	if updated.Password.Encoded() != updatedHash.Encoded() {
		t.Fatalf("updated password hash = %q, want %q", updated.Password.Encoded(), updatedHash.Encoded())
	}
	if updated.CredentialVersion != updatedVersion {
		t.Fatalf("stored credential version = %d, want %d", updated.CredentialVersion, updatedVersion)
	}

	_, err = repo.UpdatePassword(
		ctx,
		user.ID,
		valueobject.PasswordFromHash("$reject-password-update$"),
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
	if afterRollback.Password.Encoded() != updatedHash.Encoded() {
		t.Fatalf(
			"password after failed statement = %q, want unchanged %q",
			afterRollback.Password.Encoded(),
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

	filter := repository.UserListFilter{
		Roles:    []valueobject.UserRole{valueobject.RoleClient},
		Statuses: []valueobject.UserStatus{valueobject.StatusActive},
		Search:   "example.com",
	}
	firstPage, total, err := repo.List(ctx, 0, 2, filter)
	if err != nil {
		t.Fatalf("List(first page) error = %v", err)
	}
	if total != 3 {
		t.Fatalf("List(first page) total = %d, want 3", total)
	}
	assertUserIDs(t, firstPage, []string{
		"00000000-0000-4000-8000-000000000003",
		"00000000-0000-4000-8000-000000000002",
	})
	for _, user := range firstPage {
		if !user.Password.IsZero() {
			t.Fatalf("List() exposed password for user %s", user.ID)
		}
	}

	secondPage, total, err := repo.List(ctx, 2, 2, filter)
	if err != nil {
		t.Fatalf("List(second page) error = %v", err)
	}
	if total != 3 {
		t.Fatalf("List(second page) total = %d, want 3", total)
	}
	assertUserIDs(t, secondPage, []string{"00000000-0000-4000-8000-000000000001"})

	repeatedPage, repeatedTotal, err := repo.List(ctx, 0, 2, filter)
	if err != nil {
		t.Fatalf("List(repeated page) error = %v", err)
	}
	if repeatedTotal != total {
		t.Fatalf("List(repeated page) total = %d, want %d", repeatedTotal, total)
	}
	assertUserIDs(t, repeatedPage, []string{
		"00000000-0000-4000-8000-000000000003",
		"00000000-0000-4000-8000-000000000002",
	})

	empty, emptyTotal, err := repo.List(ctx, 0, 10, repository.UserListFilter{Search: "not-present"})
	if err != nil {
		t.Fatalf("List(empty) error = %v", err)
	}
	if len(empty) != 0 || emptyTotal != 0 {
		t.Fatalf("List(empty) = (%#v, %d), want empty users and total 0", empty, emptyTotal)
	}
}

func newPostgresUserIntegrationRepository(t *testing.T) *Repository {
	t.Helper()
	dsn := os.Getenv("GOAT_POSTGRES_TEST_DSN")
	if dsn == "" {
		t.Skip("set GOAT_POSTGRES_TEST_DSN to run PostgreSQL integration tests")
	}

	db, err := sqlx.Connect("postgres", dsn)
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

func integrationUser(id, email string, role valueobject.UserRole, status valueobject.UserStatus, createdAt time.Time) *entity.User {
	return &entity.User{
		ID:                id,
		Email:             email,
		Password:          valueobject.PasswordFromHash("$argon2id$integration-hash"),
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
	if got.ID != want.ID || got.Email != want.Email || got.Password.Encoded() != want.Password.Encoded() ||
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

func assertUserIDs(t *testing.T, users []*entity.User, want []string) {
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

func assertEmailConflictError(t *testing.T, err error) {
	t.Helper()
	if !errors.Is(err, domainErrors.ErrEmailAlreadyExists) {
		t.Fatalf("error = %v, want ErrEmailAlreadyExists", err)
	}
	assertPostgresConstraint(t, err, "23505", "users_email_key")
}

func assertPostgresConstraint(t *testing.T, err error, wantCode, wantConstraint string) {
	t.Helper()
	var postgresErr *pq.Error
	if !errors.As(err, &postgresErr) {
		t.Fatalf("error = %v, want preserved *pq.Error", err)
	}
	if got := string(postgresErr.Code); got != wantCode {
		t.Fatalf("PostgreSQL SQLSTATE = %q, want %q", got, wantCode)
	}
	if postgresErr.Constraint != wantConstraint {
		t.Fatalf("PostgreSQL constraint = %q, want %q", postgresErr.Constraint, wantConstraint)
	}
}
