package user

import (
	"context"
	stdErrors "errors"
	"reflect"
	"testing"

	"github.com/motixo/goat-api/internal/domain/entity"
	domainErrors "github.com/motixo/goat-api/internal/domain/errors"
	"github.com/motixo/goat-api/internal/domain/event"
	"github.com/motixo/goat-api/internal/domain/repository"
	"github.com/motixo/goat-api/internal/domain/service"
)

func TestDeleteUserSuccess(t *testing.T) {
	tests := []struct {
		name          string
		sessions      []*entity.Session
		wantDeleted   []string
		wantCallOrder []string
	}{
		{
			name:          "no active sessions",
			wantCallOrder: []string{"user.exists", "session.list", "cache.clear", "user.delete"},
		},
		{
			name:          "one active session",
			sessions:      []*entity.Session{{ID: "session-1", UserID: "user-1"}},
			wantDeleted:   []string{"session-1"},
			wantCallOrder: []string{"user.exists", "session.list", "session.delete", "cache.clear", "user.delete"},
		},
		{
			name: "multiple active sessions",
			sessions: []*entity.Session{
				{ID: "session-1", UserID: "user-1"},
				{ID: "session-2", UserID: "user-1"},
				{ID: "session-3", UserID: "user-1"},
			},
			wantDeleted:   []string{"session-1", "session-2", "session-3"},
			wantCallOrder: []string{"user.exists", "session.list", "session.delete", "cache.clear", "user.delete"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fixture := newDeletionFixture()
			fixture.sessionRepo.sessions = tt.sessions

			if err := fixture.usecase.DeleteUser(context.Background(), "user-1"); err != nil {
				t.Fatalf("DeleteUser() error = %v", err)
			}

			if !reflect.DeepEqual(fixture.recorder.calls, tt.wantCallOrder) {
				t.Fatalf("call order = %v, want %v", fixture.recorder.calls, tt.wantCallOrder)
			}
			if !reflect.DeepEqual(fixture.sessionRepo.deletedIDs, tt.wantDeleted) {
				t.Fatalf("deleted session IDs = %v, want %v", fixture.sessionRepo.deletedIDs, tt.wantDeleted)
			}
			if fixture.sessionRepo.listUserID != "user-1" {
				t.Fatalf("session lookup user ID = %q, want %q", fixture.sessionRepo.listUserID, "user-1")
			}
			if fixture.sessionRepo.listOffset != 0 || fixture.sessionRepo.listLimit != 0 {
				t.Fatalf("session lookup bounds = (%d, %d), want (0, 0)", fixture.sessionRepo.listOffset, fixture.sessionRepo.listLimit)
			}
			if !reflect.DeepEqual(fixture.cache.clearedUserIDs, []string{"user-1"}) {
				t.Fatalf("cleared user IDs = %v, want [user-1]", fixture.cache.clearedUserIDs)
			}
			if fixture.publisher.publishCalls != 0 {
				t.Fatalf("publisher calls = %d, want 0", fixture.publisher.publishCalls)
			}
		})
	}
}

func TestDeleteUserWithActiveSessionDoesNotPanic(t *testing.T) {
	fixture := newDeletionFixture()
	fixture.sessionRepo.sessions = []*entity.Session{{ID: "session-1", UserID: "user-1"}}

	var (
		gotErr    error
		recovered any
	)
	func() {
		defer func() {
			recovered = recover()
		}()
		gotErr = fixture.usecase.DeleteUser(context.Background(), "user-1")
	}()

	if recovered != nil {
		t.Fatalf("DeleteUser panicked with an active session: %v", recovered)
	}
	if gotErr != nil {
		t.Fatalf("DeleteUser() error = %v", gotErr)
	}
	wantCallOrder := []string{"user.exists", "session.list", "session.delete", "cache.clear", "user.delete"}
	if !reflect.DeepEqual(fixture.recorder.calls, wantCallOrder) {
		t.Fatalf("call order = %v, want %v", fixture.recorder.calls, wantCallOrder)
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

func TestDeleteUserSessionLookupFailureStopsBeforeDestructiveChanges(t *testing.T) {
	lookupErr := stdErrors.New("redis lookup failed")
	fixture := newDeletionFixture()
	fixture.sessionRepo.listErr = lookupErr

	err := fixture.usecase.DeleteUser(context.Background(), "user-1")

	if !stdErrors.Is(err, lookupErr) {
		t.Fatalf("DeleteUser() error = %v, want wrapped session lookup error", err)
	}
	assertDeletionCalls(t, fixture, "user.exists", "session.list")
}

func TestDeleteUserSessionDeletionFailureIsNotReportedAsSuccess(t *testing.T) {
	revocationErr := stdErrors.New("redis delete failed")
	fixture := newDeletionFixture()
	fixture.sessionRepo.sessions = []*entity.Session{{ID: "session-1", UserID: "user-1"}}
	fixture.sessionRepo.deleteErr = revocationErr

	err := fixture.usecase.DeleteUser(context.Background(), "user-1")

	if !stdErrors.Is(err, revocationErr) {
		t.Fatalf("DeleteUser() error = %v, want wrapped revocation error", err)
	}
	assertDeletionCalls(t, fixture, "user.exists", "session.list", "session.delete")
}

func TestDeleteUserCacheInvalidationFailureStopsDatabaseDeletion(t *testing.T) {
	cacheErr := stdErrors.New("redis cache delete failed")
	fixture := newDeletionFixture()
	fixture.sessionRepo.sessions = []*entity.Session{{ID: "session-1", UserID: "user-1"}}
	fixture.cache.clearErr = cacheErr

	err := fixture.usecase.DeleteUser(context.Background(), "user-1")

	if !stdErrors.Is(err, cacheErr) {
		t.Fatalf("DeleteUser() error = %v, want wrapped cache error", err)
	}
	assertDeletionCalls(t, fixture, "user.exists", "session.list", "session.delete", "cache.clear")
}

func TestDeleteUserDatabaseFailureAfterRevocationIsReturned(t *testing.T) {
	databaseErr := stdErrors.New("postgres delete failed")
	fixture := newDeletionFixture()
	fixture.sessionRepo.sessions = []*entity.Session{{ID: "session-1", UserID: "user-1"}}
	fixture.userRepo.deleteErr = databaseErr

	err := fixture.usecase.DeleteUser(context.Background(), "user-1")

	if !stdErrors.Is(err, databaseErr) {
		t.Fatalf("DeleteUser() error = %v, want database error", err)
	}
	assertDeletionCalls(t, fixture, "user.exists", "session.list", "session.delete", "cache.clear", "user.delete")
	if !reflect.DeepEqual(fixture.sessionRepo.deletedIDs, []string{"session-1"}) {
		t.Fatalf("deleted session IDs = %v, want [session-1]", fixture.sessionRepo.deletedIDs)
	}
}

func TestDeleteUserRejectsSessionWithoutServerOwnedID(t *testing.T) {
	fixture := newDeletionFixture()
	fixture.sessionRepo.sessions = []*entity.Session{{UserID: "user-1"}}

	err := fixture.usecase.DeleteUser(context.Background(), "user-1")

	if err == nil {
		t.Fatal("DeleteUser() error = nil, want invalid session error")
	}
	assertDeletionCalls(t, fixture, "user.exists", "session.list")
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
	recorder   *deletionRecorder
	sessions   []*entity.Session
	listErr    error
	deleteErr  error
	listUserID string
	listOffset int
	listLimit  int
	deletedIDs []string
}

func (r *deletionSessionRepository) ListByUser(_ context.Context, userID string, offset, limit int) ([]*entity.Session, int64, error) {
	r.recorder.record("session.list")
	r.listUserID = userID
	r.listOffset = offset
	r.listLimit = limit
	return r.sessions, int64(len(r.sessions)), r.listErr
}

func (r *deletionSessionRepository) Delete(_ context.Context, sessionIDs []string) error {
	r.recorder.record("session.delete")
	r.deletedIDs = append([]string(nil), sessionIDs...)
	return r.deleteErr
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
