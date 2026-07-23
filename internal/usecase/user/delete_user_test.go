package user

import (
	"context"
	stdErrors "errors"
	"reflect"
	"testing"
	"time"

	domainErrors "github.com/motixo/goat-api/internal/domain/errors"
	"github.com/motixo/goat-api/internal/domain/event"
	"github.com/motixo/goat-api/internal/domain/repository"
	"github.com/motixo/goat-api/internal/domain/service"
)

const expectedUserDeletionSessionCleanupTimeout = 2 * time.Second

func TestDeleteUserUsesAtomicCleanupBeforeCacheAndDatabase(t *testing.T) {
	fixture := newDeletionFixture()

	if err := fixture.usecase.DeleteUser(context.Background(), "user-1"); err != nil {
		t.Fatalf("DeleteUser() error = %v", err)
	}
	wantCallOrder := []string{"user.exists", "session.delete_all", "cache.clear", "user.delete"}
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
	if !reflect.DeepEqual(fixture.cache.clearedUserIDs, []string{"user-1"}) {
		t.Fatalf("cleared user IDs = %v, want [user-1]", fixture.cache.clearedUserIDs)
	}
	if fixture.publisher.publishCalls != 0 {
		t.Fatalf("publisher calls = %d, want 0", fixture.publisher.publishCalls)
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

func TestDeleteUserAtomicSessionCleanupFailureStopsBeforeCacheAndDatabase(t *testing.T) {
	cleanupErr := stdErrors.New("redis cleanup failed")
	fixture := newDeletionFixture()
	fixture.sessionRepo.deleteAllErr = cleanupErr

	err := fixture.usecase.DeleteUser(context.Background(), "user-1")

	if !stdErrors.Is(err, cleanupErr) {
		t.Fatalf("DeleteUser() error = %v, want wrapped cleanup error", err)
	}
	assertDeletionCalls(t, fixture, "user.exists", "session.delete_all")
}

func TestDeleteUserAtomicSessionCleanupTimeoutStopsBeforeCacheAndDatabase(t *testing.T) {
	fixture := newDeletionFixture()
	fixture.sessionRepo.waitForContext = true

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err := fixture.usecase.DeleteUser(ctx, "user-1")

	if !stdErrors.Is(err, context.Canceled) {
		t.Fatalf("DeleteUser() error = %v, want wrapped context cancellation", err)
	}
	assertDeletionCalls(t, fixture, "user.exists", "session.delete_all")
}

func TestDeleteUserCacheInvalidationFailureStopsDatabaseDeletion(t *testing.T) {
	cacheErr := stdErrors.New("redis cache delete failed")
	fixture := newDeletionFixture()
	fixture.cache.clearErr = cacheErr

	err := fixture.usecase.DeleteUser(context.Background(), "user-1")

	if !stdErrors.Is(err, cacheErr) {
		t.Fatalf("DeleteUser() error = %v, want wrapped cache error", err)
	}
	assertDeletionCalls(t, fixture, "user.exists", "session.delete_all", "cache.clear")
}

func TestDeleteUserDatabaseFailureAfterRevocationIsReturned(t *testing.T) {
	databaseErr := stdErrors.New("postgres delete failed")
	fixture := newDeletionFixture()
	fixture.userRepo.deleteErr = databaseErr

	err := fixture.usecase.DeleteUser(context.Background(), "user-1")

	if !stdErrors.Is(err, databaseErr) {
		t.Fatalf("DeleteUser() error = %v, want database error", err)
	}
	assertDeletionCalls(t, fixture, "user.exists", "session.delete_all", "cache.clear", "user.delete")
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
	cache       *deletionUserCache
	publisher   *deletionPublisher
	usecase     UseCase
}

func newDeletionFixture() *deletionFixture {
	recorder := &deletionRecorder{}
	userRepo := &deletionUserRepository{recorder: recorder, exists: true}
	sessionRepo := &deletionSessionRepository{recorder: recorder}
	cache := &deletionUserCache{recorder: recorder}
	publisher := &deletionPublisher{recorder: recorder}

	return &deletionFixture{
		recorder:    recorder,
		userRepo:    userRepo,
		sessionRepo: sessionRepo,
		cache:       cache,
		publisher:   publisher,
		usecase: NewUsecase(
			userRepo,
			nil,
			discardUserLogger{},
			sessionRepo,
			cache,
			publisher,
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

func (r *deletionSessionRepository) DeleteAllByUser(ctx context.Context, userID string) error {
	r.recorder.record("session.delete_all")
	r.deletedUserID = userID
	r.deadline, r.hasDeadline = ctx.Deadline()
	if r.waitForContext {
		<-ctx.Done()
		return ctx.Err()
	}
	return r.deleteAllErr
}

type deletionUserCache struct {
	service.UserCacheService
	recorder       *deletionRecorder
	clearErr       error
	clearedUserIDs []string
}

func (c *deletionUserCache) ClearCache(_ context.Context, userID string) error {
	c.recorder.record("cache.clear")
	c.clearedUserIDs = append(c.clearedUserIDs, userID)
	return c.clearErr
}

type deletionPublisher struct {
	event.Publisher
	recorder     *deletionRecorder
	publishCalls int
}

func (p *deletionPublisher) Publish(context.Context, any) error {
	p.recorder.record("publisher.publish")
	p.publishCalls++
	return nil
}

type discardUserLogger struct{}

func (discardUserLogger) Info(string, ...any)  {}
func (discardUserLogger) Error(string, ...any) {}
func (discardUserLogger) Warn(string, ...any)  {}
func (discardUserLogger) Debug(string, ...any) {}
func (discardUserLogger) Panic(string, ...any) {}
