package user

import (
	"context"
	stdErrors "errors"
	"reflect"
	"testing"
	"time"

	domainErrors "github.com/motixo/goat-api/internal/domain/errors"
	"github.com/motixo/goat-api/internal/domain/repository"
)

const expectedUserDeletionSessionCleanupTimeout = 2 * time.Second

func TestDeleteUserUsesAtomicCleanupBeforeDatabase(t *testing.T) {
	fixture := newDeletionFixture()

	if err := fixture.usecase.DeleteUser(context.Background(), "user-1"); err != nil {
		t.Fatalf("DeleteUser() error = %v", err)
	}
	wantCallOrder := []string{"user.exists", "session.block_delete_all", "user.delete"}
	if !reflect.DeepEqual(fixture.recorder.calls, wantCallOrder) {
		t.Fatalf("call order = %v, want %v", fixture.recorder.calls, wantCallOrder)
	}
	if fixture.sessionRepo.deletedUserID != "user-1" {
		t.Fatalf("atomic cleanup user ID = %q, want user-1", fixture.sessionRepo.deletedUserID)
	}
	if !fixture.sessionRepo.hasDeadline {
		t.Fatal("atomic cleanup context has no deadline")
	}
	cleanupBudget := time.Until(fixture.sessionRepo.deadline)
	if cleanupBudget <= 0 || cleanupBudget > expectedUserDeletionSessionCleanupTimeout {
		t.Fatalf(
			"atomic cleanup deadline budget = %s, want within (0, %s]",
			cleanupBudget,
			expectedUserDeletionSessionCleanupTimeout,
		)
	}
}

func TestDeleteUserNotFound(t *testing.T) {
	fixture := newDeletionFixture()
	fixture.userRepo.exists = false

	err := fixture.usecase.DeleteUser(context.Background(), "missing-user")

	if !stdErrors.Is(err, domainErrors.ErrUserNotFound) {
		t.Fatalf("DeleteUser() error = %v, want ErrUserNotFound", err)
	}
	assertDeletionCalls(t, fixture, "user.exists")
}

func TestDeleteUserExistenceLookupFailureStopsBeforeDestructiveChanges(t *testing.T) {
	lookupErr := stdErrors.New("postgres unavailable")
	fixture := newDeletionFixture()
	fixture.userRepo.existsErr = lookupErr

	err := fixture.usecase.DeleteUser(context.Background(), "user-1")

	if !stdErrors.Is(err, lookupErr) {
		t.Fatalf("DeleteUser() error = %v, want wrapped lookup error", err)
	}
	assertDeletionCalls(t, fixture, "user.exists")
}

func TestDeleteUserAtomicSessionCleanupFailureStopsBeforeDatabase(t *testing.T) {
	cleanupErr := stdErrors.New("redis cleanup failed")
	fixture := newDeletionFixture()
	fixture.sessionRepo.deleteAllErr = cleanupErr

	err := fixture.usecase.DeleteUser(context.Background(), "user-1")

	if !stdErrors.Is(err, cleanupErr) {
		t.Fatalf("DeleteUser() error = %v, want wrapped cleanup error", err)
	}
	assertDeletionCalls(t, fixture, "user.exists", "session.block_delete_all")
}

func TestDeleteUserAtomicSessionCleanupTimeoutStopsBeforeDatabase(t *testing.T) {
	fixture := newDeletionFixture()
	fixture.sessionRepo.waitForContext = true

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err := fixture.usecase.DeleteUser(ctx, "user-1")

	if !stdErrors.Is(err, context.Canceled) {
		t.Fatalf("DeleteUser() error = %v, want wrapped context cancellation", err)
	}
	assertDeletionCalls(t, fixture, "user.exists", "session.block_delete_all")
}

func TestDeleteUserDatabaseFailureAfterRevocationIsReturned(t *testing.T) {
	databaseErr := stdErrors.New("postgres delete failed")
	fixture := newDeletionFixture()
	fixture.userRepo.deleteErr = databaseErr

	err := fixture.usecase.DeleteUser(context.Background(), "user-1")

	if !stdErrors.Is(err, databaseErr) {
		t.Fatalf("DeleteUser() error = %v, want database error", err)
	}
	assertDeletionCalls(t, fixture, "user.exists", "session.block_delete_all", "user.delete")
	if fixture.sessionRepo.deletedUserID != "user-1" {
		t.Fatalf("atomic cleanup user ID = %q, want user-1", fixture.sessionRepo.deletedUserID)
	}
}

func assertDeletionCalls(t *testing.T, fixture *deletionFixture, want ...string) {
	t.Helper()
	if !reflect.DeepEqual(fixture.recorder.calls, want) {
		t.Fatalf("call order = %v, want %v", fixture.recorder.calls, want)
	}
}

type deletionFixture struct {
	recorder    *deletionRecorder
	userRepo    *deletionUserRepository
	sessionRepo *deletionSessionRepository
	usecase     UseCase
}

func newDeletionFixture() *deletionFixture {
	recorder := &deletionRecorder{}
	userRepo := &deletionUserRepository{recorder: recorder, exists: true}
	sessionRepo := &deletionSessionRepository{recorder: recorder}

	return &deletionFixture{
		recorder:    recorder,
		userRepo:    userRepo,
		sessionRepo: sessionRepo,
		usecase: NewUsecase(
			userRepo,
			nil,
			discardUserLogger{},
			sessionRepo,
			nil,
		),
	}
}

type deletionRecorder struct {
	calls []string
}

func (r *deletionRecorder) record(call string) {
	r.calls = append(r.calls, call)
}

type deletionUserRepository struct {
	repository.UserRepository
	recorder  *deletionRecorder
	exists    bool
	existsErr error
	deleteErr error
}

func (r *deletionUserRepository) ExistsByID(context.Context, string) (bool, error) {
	r.recorder.record("user.exists")
	return r.exists, r.existsErr
}

func (r *deletionUserRepository) Delete(context.Context, string) error {
	r.recorder.record("user.delete")
	return r.deleteErr
}

type deletionSessionRepository struct {
	repository.SessionRepository
	recorder       *deletionRecorder
	deleteAllErr   error
	deletedUserID  string
	deadline       time.Time
	hasDeadline    bool
	waitForContext bool
}

func (r *deletionSessionRepository) BlockAndDeleteAllByUser(
	ctx context.Context,
	userID string,
) error {
	r.recorder.record("session.block_delete_all")
	r.deletedUserID = userID
	r.deadline, r.hasDeadline = ctx.Deadline()
	if r.waitForContext {
		<-ctx.Done()
		return ctx.Err()
	}
	return r.deleteAllErr
}

type discardUserLogger struct{}

func (discardUserLogger) Info(string, ...any)  {}
func (discardUserLogger) Error(string, ...any) {}
func (discardUserLogger) Warn(string, ...any)  {}
func (discardUserLogger) Debug(string, ...any) {}
func (discardUserLogger) Panic(string, ...any) {}
