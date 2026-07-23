package user

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/motixo/goat-api/internal/domain/entity"
	domainErrors "github.com/motixo/goat-api/internal/domain/errors"
	"github.com/motixo/goat-api/internal/domain/repository"
	"github.com/motixo/goat-api/internal/domain/service"
	"github.com/motixo/goat-api/internal/domain/valueobject"
)

const (
	passwordChangeUserID      = "user-1"
	passwordChangeOldPassword = "OldPassword1!"
	passwordChangeNewPassword = "NewPassword1!"
	passwordChangeOldHash     = "old-password-hash"
	passwordChangeNewHash     = "new-password-hash"
)

func TestChangePasswordSuccessCommitsVersionBeforeAtomicSessionCleanup(t *testing.T) {
	fixture := newPasswordChangeFixture()

	err := fixture.usecase.ChangePassword(context.Background(), passwordChangeInput())

	if err != nil {
		t.Fatalf("ChangePassword() error = %v", err)
	}
	assertPasswordChangeCalls(t, fixture,
		"user.find",
		"password.verify",
		"password.hash",
		"user.update_password",
		"session.delete_all",
	)
	if fixture.sessionRepo.deleteAllUserID != passwordChangeUserID {
		t.Fatalf(
			"atomic cleanup user ID = %q, want %q",
			fixture.sessionRepo.deleteAllUserID,
			passwordChangeUserID,
		)
	}
	if !fixture.sessionRepo.deleteAllHasDeadline {
		t.Fatal("atomic cleanup context has no deadline")
	}
	remaining := time.Until(fixture.sessionRepo.deleteAllDeadline)
	if remaining <= 0 || remaining > passwordChangeSessionCleanupTimeout {
		t.Fatalf(
			"atomic cleanup deadline remaining = %s, want > 0 and <= %s",
			remaining,
			passwordChangeSessionCleanupTimeout,
		)
	}
	if len(fixture.cache.clearedUserIDs) != 0 {
		t.Fatalf("authorization cache was cleared for password-only change: %v", fixture.cache.clearedUserIDs)
	}
	if fixture.userRepo.persistedPassword.Encoded() != passwordChangeNewHash {
		t.Fatalf("persisted password = %q, want %q", fixture.userRepo.persistedPassword.Encoded(), passwordChangeNewHash)
	}
	if fixture.userRepo.credentialVersion != 2 {
		t.Fatalf("credential version = %d, want 2", fixture.userRepo.credentialVersion)
	}
}

func TestChangePasswordIdempotentCleanupLeavesAuthorizationCacheAlone(t *testing.T) {
	fixture := newPasswordChangeFixture()

	err := fixture.usecase.ChangePassword(context.Background(), passwordChangeInput())

	if err != nil {
		t.Fatalf("ChangePassword() error = %v", err)
	}
	assertPasswordChangeCalls(t, fixture,
		"user.find",
		"password.verify",
		"password.hash",
		"user.update_password",
		"session.delete_all",
	)
	if len(fixture.cache.clearedUserIDs) != 0 {
		t.Fatalf("authorization cache was cleared for password-only change: %v", fixture.cache.clearedUserIDs)
	}
}

func TestChangePasswordIncorrectCurrentPasswordStopsBeforeDestructiveOperations(t *testing.T) {
	fixture := newPasswordChangeFixture()
	fixture.passwordHasher.verifyResult = false

	err := fixture.usecase.ChangePassword(context.Background(), passwordChangeInput())

	if !errors.Is(err, domainErrors.ErrInvalidPassword) {
		t.Fatalf("ChangePassword() error = %v, want ErrInvalidPassword", err)
	}
	assertPasswordChangeCalls(t, fixture, "user.find", "password.verify")
	assertPasswordUnchanged(t, fixture)
}

func TestChangePasswordInvalidNewPasswordStopsBeforeDestructiveOperations(t *testing.T) {
	fixture := newPasswordChangeFixture()
	fixture.passwordHasher.hashErr = domainErrors.ErrPasswordTooShort

	err := fixture.usecase.ChangePassword(context.Background(), passwordChangeInput())

	if !errors.Is(err, domainErrors.ErrPasswordTooShort) {
		t.Fatalf("ChangePassword() error = %v, want ErrPasswordTooShort", err)
	}
	assertPasswordChangeCalls(t, fixture, "user.find", "password.verify", "password.hash")
	assertPasswordUnchanged(t, fixture)
}

func TestChangePasswordUserNotFoundStopsBeforePasswordVerification(t *testing.T) {
	fixture := newPasswordChangeFixture()
	fixture.userRepo.findErr = domainErrors.ErrUserNotFound

	err := fixture.usecase.ChangePassword(context.Background(), passwordChangeInput())

	if !errors.Is(err, domainErrors.ErrUserNotFound) {
		t.Fatalf("ChangePassword() error = %v, want ErrUserNotFound", err)
	}
	assertPasswordChangeCalls(t, fixture, "user.find")
	assertPasswordUnchanged(t, fixture)
}

func TestChangePasswordAtomicSessionCleanupFailureAfterCommitStillReturnsSuccess(t *testing.T) {
	cleanupErr := errors.New("redis atomic session cleanup failed")
	fixture := newPasswordChangeFixture()
	fixture.sessionRepo.deleteAllErr = cleanupErr

	err := fixture.usecase.ChangePassword(context.Background(), passwordChangeInput())

	if err != nil {
		t.Fatalf("ChangePassword() error = %v after committed password update", err)
	}
	assertPasswordChangeCalls(t, fixture,
		"user.find",
		"password.verify",
		"password.hash",
		"user.update_password",
		"session.delete_all",
	)
	assertPasswordAndVersionChanged(t, fixture)
	assertPasswordCleanupFailureObserved(
		t,
		fixture,
		passwordChangeCleanupStageSessionRevocation,
		cleanupErr,
	)
}

func TestChangePasswordAtomicSessionCleanupTimeoutAfterCommitStillReturnsSuccess(t *testing.T) {
	fixture := newPasswordChangeFixture()
	fixture.sessionRepo.deleteAll = func(ctx context.Context, _ string) error {
		<-ctx.Done()
		return ctx.Err()
	}

	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Millisecond)
	defer cancel()
	startedAt := time.Now()
	err := fixture.usecase.ChangePassword(ctx, passwordChangeInput())

	if err != nil {
		t.Fatalf("ChangePassword() error = %v after committed password update", err)
	}
	if elapsed := time.Since(startedAt); elapsed > time.Second {
		t.Fatalf("ChangePassword() took %s after cleanup timeout, want < 1s", elapsed)
	}
	assertPasswordChangeCalls(t, fixture,
		"user.find",
		"password.verify",
		"password.hash",
		"user.update_password",
		"session.delete_all",
	)
	assertPasswordAndVersionChanged(t, fixture)
	assertPasswordCleanupFailureObserved(
		t,
		fixture,
		passwordChangeCleanupStageSessionRevocation,
		context.DeadlineExceeded,
	)
}

func TestChangePasswordDoesNotInvalidateRoleStatusAuthorizationCache(t *testing.T) {
	cacheErr := errors.New("redis cache invalidation failed")
	fixture := newPasswordChangeFixture()
	fixture.cache.clearErr = cacheErr

	err := fixture.usecase.ChangePassword(context.Background(), passwordChangeInput())

	if err != nil {
		t.Fatalf("ChangePassword() error = %v with irrelevant cache failure configured", err)
	}
	assertPasswordChangeCalls(t, fixture,
		"user.find",
		"password.verify",
		"password.hash",
		"user.update_password",
		"session.delete_all",
	)
	assertPasswordAndVersionChanged(t, fixture)
	if len(fixture.cache.clearedUserIDs) != 0 {
		t.Fatalf("authorization cache was cleared for password-only change: %v", fixture.cache.clearedUserIDs)
	}
	if len(fixture.metrics.stages) != 0 {
		t.Fatalf("cleanup failure metrics = %v, want none", fixture.metrics.stages)
	}
}

func TestChangePasswordDatabaseFailureDoesNotIncrementVersionOrStartRedisCleanup(t *testing.T) {
	updateErr := errors.New("postgres password update failed")
	fixture := newPasswordChangeFixture()
	fixture.userRepo.updatePasswordErr = updateErr

	err := fixture.usecase.ChangePassword(context.Background(), passwordChangeInput())

	if !errors.Is(err, updateErr) {
		t.Fatalf("ChangePassword() error = %v, want PostgreSQL update error", err)
	}
	assertPasswordChangeCalls(t, fixture,
		"user.find",
		"password.verify",
		"password.hash",
		"user.update_password",
	)
	assertPasswordUnchanged(t, fixture)
	if fixture.userRepo.credentialVersion != entity.InitialCredentialVersion {
		t.Fatalf(
			"credential version after failed update = %d, want %d",
			fixture.userRepo.credentialVersion,
			entity.InitialCredentialVersion,
		)
	}
}

func TestChangePasswordRepeatedRequestUsesCommittedPasswordState(t *testing.T) {
	fixture := newPasswordChangeFixture()
	fixture.passwordHasher.verify = func(password string, hash valueobject.Password) bool {
		return password == passwordChangeOldPassword && hash.Encoded() == passwordChangeOldHash
	}

	if err := fixture.usecase.ChangePassword(context.Background(), passwordChangeInput()); err != nil {
		t.Fatalf("first ChangePassword() error = %v", err)
	}
	err := fixture.usecase.ChangePassword(context.Background(), passwordChangeInput())

	if !errors.Is(err, domainErrors.ErrInvalidPassword) {
		t.Fatalf("second ChangePassword() error = %v, want ErrInvalidPassword", err)
	}
	if fixture.userRepo.credentialVersion != entity.InitialCredentialVersion+1 {
		t.Fatalf(
			"credential version after repeated request = %d, want %d",
			fixture.userRepo.credentialVersion,
			entity.InitialCredentialVersion+1,
		)
	}
	if fixture.userRepo.persistedPassword.Encoded() != passwordChangeNewHash {
		t.Fatalf("persisted password = %q, want %q", fixture.userRepo.persistedPassword.Encoded(), passwordChangeNewHash)
	}
}

func passwordChangeInput() UpdatePassInput {
	return UpdatePassInput{
		UserID:      passwordChangeUserID,
		OldPassword: passwordChangeOldPassword,
		NewPassword: passwordChangeNewPassword,
	}
}

func assertPasswordChangeCalls(t *testing.T, fixture *passwordChangeFixture, want ...string) {
	t.Helper()
	if !reflect.DeepEqual(fixture.recorder.calls, want) {
		t.Fatalf("call order = %v, want %v", fixture.recorder.calls, want)
	}
}

func assertPasswordUnchanged(t *testing.T, fixture *passwordChangeFixture) {
	t.Helper()
	if fixture.userRepo.persistedPassword.Encoded() != passwordChangeOldHash {
		t.Fatalf("persisted password = %q, want unchanged %q",
			fixture.userRepo.persistedPassword.Encoded(),
			passwordChangeOldHash,
		)
	}
}

func assertPasswordAndVersionChanged(t *testing.T, fixture *passwordChangeFixture) {
	t.Helper()
	if fixture.userRepo.persistedPassword.Encoded() != passwordChangeNewHash {
		t.Fatalf(
			"persisted password = %q, want changed %q",
			fixture.userRepo.persistedPassword.Encoded(),
			passwordChangeNewHash,
		)
	}
	if fixture.userRepo.credentialVersion != entity.InitialCredentialVersion+1 {
		t.Fatalf(
			"credential version = %d, want %d",
			fixture.userRepo.credentialVersion,
			entity.InitialCredentialVersion+1,
		)
	}
}

func assertPasswordCleanupFailureObserved(
	t *testing.T,
	fixture *passwordChangeFixture,
	wantStage string,
	wantErr error,
) {
	t.Helper()
	if len(fixture.logger.errors) != 1 {
		t.Fatalf("cleanup error log count = %d, want 1", len(fixture.logger.errors))
	}
	entry := fixture.logger.errors[0]
	if entry.message != "post-commit password-change session cleanup failed" {
		t.Fatalf("cleanup error message = %q", entry.message)
	}
	fields := make(map[string]any, len(entry.fields)/2)
	for index := 0; index+1 < len(entry.fields); index += 2 {
		key, ok := entry.fields[index].(string)
		if ok {
			fields[key] = entry.fields[index+1]
		}
	}
	if fields["cleanup_stage"] != wantStage {
		t.Fatalf("cleanup stage = %#v, want %q", fields["cleanup_stage"], wantStage)
	}
	if fields["credential_change_committed"] != true {
		t.Fatalf("credential_change_committed = %#v, want true", fields["credential_change_committed"])
	}
	if wantErr != nil {
		loggedErr, ok := fields["error"].(error)
		if !ok || !errors.Is(loggedErr, wantErr) {
			t.Fatalf("logged cleanup error = %v, want %v", fields["error"], wantErr)
		}
	}
	if !reflect.DeepEqual(fixture.metrics.stages, []string{wantStage}) {
		t.Fatalf("cleanup failure metric stages = %v, want [%s]", fixture.metrics.stages, wantStage)
	}
}

type passwordChangeFixture struct {
	recorder       *passwordChangeRecorder
	userRepo       *passwordChangeUserRepository
	passwordHasher *passwordChangeHasher
	sessionRepo    *passwordChangeSessionRepository
	cache          *passwordChangeUserCache
	logger         *passwordChangeLogRecorder
	metrics        *passwordChangeCleanupMetrics
	usecase        UseCase
}

func newPasswordChangeFixture() *passwordChangeFixture {
	recorder := &passwordChangeRecorder{}
	logRecorder := &passwordChangeLogRecorder{}
	oldHash := valueobject.PasswordFromHash(passwordChangeOldHash)
	userRepo := &passwordChangeUserRepository{
		recorder: recorder,
		user: &entity.User{
			ID:                passwordChangeUserID,
			Password:          oldHash,
			CredentialVersion: entity.InitialCredentialVersion,
		},
		persistedPassword: oldHash,
		credentialVersion: entity.InitialCredentialVersion,
	}
	passwordHasher := &passwordChangeHasher{
		recorder:     recorder,
		verifyResult: true,
		hash:         valueobject.PasswordFromHash(passwordChangeNewHash),
	}
	sessionRepo := &passwordChangeSessionRepository{recorder: recorder}
	cache := &passwordChangeUserCache{recorder: recorder}
	cleanupMetrics := &passwordChangeCleanupMetrics{}

	return &passwordChangeFixture{
		recorder:       recorder,
		userRepo:       userRepo,
		passwordHasher: passwordHasher,
		sessionRepo:    sessionRepo,
		cache:          cache,
		logger:         logRecorder,
		metrics:        cleanupMetrics,
		usecase: NewUsecase(
			userRepo,
			passwordHasher,
			passwordChangeLogger{recorder: logRecorder},
			sessionRepo,
			cache,
			nil,
			cleanupMetrics,
		),
	}
}

type passwordChangeRecorder struct {
	calls []string
}

func (r *passwordChangeRecorder) record(call string) {
	r.calls = append(r.calls, call)
}

type passwordChangeCleanupMetrics struct {
	stages []string
}

func (m *passwordChangeCleanupMetrics) RecordPasswordChangeCleanupFailure(stage string) {
	m.stages = append(m.stages, stage)
}

type passwordChangeUserRepository struct {
	repository.UserRepository
	recorder          *passwordChangeRecorder
	user              *entity.User
	findErr           error
	updatePasswordErr error
	persistedPassword valueobject.Password
	credentialVersion int64
}

func (r *passwordChangeUserRepository) FindByID(context.Context, string) (*entity.User, error) {
	r.recorder.record("user.find")
	return r.user, r.findErr
}

func (r *passwordChangeUserRepository) UpdatePassword(
	_ context.Context,
	_ string,
	password valueobject.Password,
) (int64, error) {
	r.recorder.record("user.update_password")
	if r.updatePasswordErr != nil {
		return 0, r.updatePasswordErr
	}
	r.persistedPassword = password
	r.credentialVersion++
	r.user.Password = password
	r.user.CredentialVersion = r.credentialVersion
	return r.credentialVersion, nil
}

type passwordChangeHasher struct {
	service.PasswordHasher
	recorder     *passwordChangeRecorder
	verifyResult bool
	verify       func(password string, hash valueobject.Password) bool
	hash         valueobject.Password
	hashErr      error
}

func (h *passwordChangeHasher) Verify(_ context.Context, password string, hash valueobject.Password) bool {
	h.recorder.record("password.verify")
	if h.verify != nil {
		return h.verify(password, hash)
	}
	return h.verifyResult
}

func (h *passwordChangeHasher) Hash(context.Context, string) (valueobject.Password, error) {
	h.recorder.record("password.hash")
	return h.hash, h.hashErr
}

type passwordChangeSessionRepository struct {
	repository.SessionRepository
	recorder             *passwordChangeRecorder
	sessions             []*entity.Session
	listErr              error
	deleteErr            error
	listUserID           string
	listOffset           int
	listLimit            int
	deletedIDs           []string
	deleteAll            func(context.Context, string) error
	deleteAllErr         error
	deleteAllUserID      string
	deleteAllDeadline    time.Time
	deleteAllHasDeadline bool
}

func (r *passwordChangeSessionRepository) ListByUser(
	_ context.Context,
	userID string,
	offset, limit int,
) ([]*entity.Session, int64, error) {
	r.recorder.record("session.list")
	r.listUserID = userID
	r.listOffset = offset
	r.listLimit = limit
	return r.sessions, int64(len(r.sessions)), r.listErr
}

func (r *passwordChangeSessionRepository) Delete(_ context.Context, sessionIDs []string) error {
	r.recorder.record("session.delete")
	r.deletedIDs = append([]string(nil), sessionIDs...)
	return r.deleteErr
}

func (r *passwordChangeSessionRepository) DeleteAllByUser(ctx context.Context, userID string) error {
	r.recorder.record("session.delete_all")
	r.deleteAllUserID = userID
	r.deleteAllDeadline, r.deleteAllHasDeadline = ctx.Deadline()
	if r.deleteAll != nil {
		return r.deleteAll(ctx, userID)
	}
	return r.deleteAllErr
}

type passwordChangeUserCache struct {
	service.UserCacheService
	recorder       *passwordChangeRecorder
	clearErr       error
	clearedUserIDs []string
}

func (c *passwordChangeUserCache) ClearCache(_ context.Context, userID string) error {
	c.recorder.record("cache.clear")
	c.clearedUserIDs = append(c.clearedUserIDs, userID)
	return c.clearErr
}

type passwordChangeLogEntry struct {
	message string
	fields  []any
}

type passwordChangeLogRecorder struct {
	errors []passwordChangeLogEntry
}

type passwordChangeLogger struct {
	recorder *passwordChangeLogRecorder
}

func (passwordChangeLogger) Info(string, ...any) {}

func (l passwordChangeLogger) Error(message string, fields ...any) {
	if l.recorder == nil {
		return
	}
	l.recorder.errors = append(l.recorder.errors, passwordChangeLogEntry{
		message: message,
		fields:  append([]any(nil), fields...),
	})
}

func (passwordChangeLogger) Warn(string, ...any)  {}
func (passwordChangeLogger) Debug(string, ...any) {}
func (passwordChangeLogger) Panic(string, ...any) {}
