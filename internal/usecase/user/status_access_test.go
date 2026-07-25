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
	"github.com/motixo/goat-api/internal/domain/valueobject"
)

func TestSuspendUserBlocksAccessBeforePostgreSQL(t *testing.T) {
	fixture := newStatusAccessFixture()

	err := fixture.usecase.ChangeStatus(context.Background(), statusChangeInput(
		valueobject.StatusSuspended,
	))

	if err != nil {
		t.Fatalf("ChangeStatus() error = %v", err)
	}
	assertStatusAccessCalls(
		t,
		fixture,
		"user.find",
		"session.block_delete_all",
		"user.update_status",
	)
	if !fixture.sessions.blockHasDeadline {
		t.Fatal("block-and-delete context has no deadline")
	}
	if remaining := time.Until(fixture.sessions.blockDeadline); remaining <= 0 ||
		remaining > userStatusAccessStateTimeout {
		t.Fatalf(
			"block-and-delete deadline = %s, want within (0, %s]",
			remaining,
			userStatusAccessStateTimeout,
		)
	}
}

func TestSuspendUserRedisFailureLeavesPostgreSQLUntouched(t *testing.T) {
	fixture := newStatusAccessFixture()
	redisErr := errors.New("redis unavailable")
	fixture.sessions.blockErr = redisErr

	err := fixture.usecase.ChangeStatus(context.Background(), statusChangeInput(
		valueobject.StatusSuspended,
	))

	if !errors.Is(err, redisErr) {
		t.Fatalf("ChangeStatus() error = %v, want Redis failure", err)
	}
	assertStatusAccessCalls(t, fixture, "user.find", "session.block_delete_all")
	if fixture.users.updatedStatus != valueobject.StatusUnknown {
		t.Fatalf("PostgreSQL status changed to %s after Redis failure", fixture.users.updatedStatus)
	}
}

func TestSuspendUserPostgreSQLFailureLeavesRedisBlockApplied(t *testing.T) {
	fixture := newStatusAccessFixture()
	postgresErr := errors.New("postgres unavailable")
	fixture.users.updateErr = postgresErr

	err := fixture.usecase.ChangeStatus(context.Background(), statusChangeInput(
		valueobject.StatusSuspended,
	))

	if !errors.Is(err, postgresErr) {
		t.Fatalf("ChangeStatus() error = %v, want PostgreSQL failure", err)
	}
	assertStatusAccessCalls(
		t,
		fixture,
		"user.find",
		"session.block_delete_all",
		"user.update_status",
	)
	if fixture.sessions.blockCalls != 1 {
		t.Fatalf("Redis blocks = %d, want 1", fixture.sessions.blockCalls)
	}
}

func TestReactivateUserUnblocksBeforePostgreSQL(t *testing.T) {
	fixture := newStatusAccessFixture()
	fixture.users.target.Status = valueobject.StatusSuspended

	err := fixture.usecase.ChangeStatus(context.Background(), statusChangeInput(
		valueobject.StatusActive,
	))

	if err != nil {
		t.Fatalf("ChangeStatus() error = %v", err)
	}
	assertStatusAccessCalls(
		t,
		fixture,
		"user.find",
		"session.unblock",
		"user.update_status",
	)
	if !fixture.sessions.unblockHasDeadline {
		t.Fatal("unblock context has no deadline")
	}
}

func TestReactivateUserRedisFailureLeavesPostgreSQLSuspended(t *testing.T) {
	fixture := newStatusAccessFixture()
	fixture.users.target.Status = valueobject.StatusSuspended
	redisErr := errors.New("redis unavailable")
	fixture.sessions.unblockErr = redisErr

	err := fixture.usecase.ChangeStatus(context.Background(), statusChangeInput(
		valueobject.StatusActive,
	))

	if !errors.Is(err, redisErr) {
		t.Fatalf("ChangeStatus() error = %v, want Redis failure", err)
	}
	assertStatusAccessCalls(t, fixture, "user.find", "session.unblock")
	if fixture.users.updatedStatus != valueobject.StatusUnknown {
		t.Fatalf(
			"PostgreSQL status changed to %s after Redis failure",
			fixture.users.updatedStatus,
		)
	}
}

func TestReactivateUserPostgreSQLFailureAfterUnblockIsRetryable(t *testing.T) {
	fixture := newStatusAccessFixture()
	fixture.users.target.Status = valueobject.StatusSuspended
	postgresErr := errors.New("postgres unavailable")
	fixture.users.updateErr = postgresErr

	err := fixture.usecase.ChangeStatus(context.Background(), statusChangeInput(
		valueobject.StatusActive,
	))
	if !errors.Is(err, postgresErr) {
		t.Fatalf("first ChangeStatus() error = %v, want PostgreSQL failure", err)
	}
	assertStatusAccessCalls(
		t,
		fixture,
		"user.find",
		"session.unblock",
		"user.update_status",
	)
	if fixture.users.target.Status != valueobject.StatusSuspended {
		t.Fatalf(
			"durable status = %s after PostgreSQL failure, want suspended",
			fixture.users.target.Status,
		)
	}

	fixture.recorder.calls = nil
	fixture.users.updateErr = nil
	if err := fixture.usecase.ChangeStatus(
		context.Background(),
		statusChangeInput(valueobject.StatusActive),
	); err != nil {
		t.Fatalf("retry ChangeStatus() error = %v", err)
	}
	assertStatusAccessCalls(
		t,
		fixture,
		"user.find",
		"session.unblock",
		"user.update_status",
	)
	if fixture.sessions.unblockCalls != 2 {
		t.Fatalf("Redis unblocks = %d, want 2 idempotent attempts", fixture.sessions.unblockCalls)
	}
}

func TestActivateInactiveUserPersistsWithoutInitializingRedisAccessState(t *testing.T) {
	fixture := newStatusAccessFixture()
	fixture.users.target.Status = valueobject.StatusInactive

	if err := fixture.usecase.ChangeStatus(
		context.Background(),
		statusChangeInput(valueobject.StatusActive),
	); err != nil {
		t.Fatalf("ChangeStatus() error = %v", err)
	}
	assertStatusAccessCalls(
		t,
		fixture,
		"user.find",
		"user.update_status",
	)
	if fixture.sessions.blockCalls != 0 || fixture.sessions.unblockCalls != 0 {
		t.Fatalf(
			"Redis access-state calls: block=%d unblock=%d, want 0/0",
			fixture.sessions.blockCalls,
			fixture.sessions.unblockCalls,
		)
	}
}

func TestForbiddenStatusTransitionsDoNotMutatePostgreSQLOrRedis(t *testing.T) {
	tests := []struct {
		name    string
		current valueobject.UserStatus
		next    valueobject.UserStatus
	}{
		{
			name:    "active to inactive",
			current: valueobject.StatusActive,
			next:    valueobject.StatusInactive,
		},
		{
			name:    "suspended to inactive",
			current: valueobject.StatusSuspended,
			next:    valueobject.StatusInactive,
		},
		{
			name:    "inactive to suspended",
			current: valueobject.StatusInactive,
			next:    valueobject.StatusSuspended,
		},
		{
			name:    "unknown current status",
			current: valueobject.StatusUnknown,
			next:    valueobject.StatusActive,
		},
		{
			name:    "unknown target status",
			current: valueobject.StatusActive,
			next:    valueobject.StatusUnknown,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			fixture := newStatusAccessFixture()
			fixture.users.target.Status = test.current

			err := fixture.usecase.ChangeStatus(
				context.Background(),
				statusChangeInput(test.next),
			)

			if err == nil {
				t.Fatal("ChangeStatus() error = nil, want rejected transition")
			}
			if test.current.IsKnown() &&
				!errors.Is(err, domainErrors.ErrInvalidUserStatusTransition) {
				t.Fatalf(
					"ChangeStatus() error = %v, want ErrInvalidUserStatusTransition",
					err,
				)
			}
			assertStatusAccessCalls(t, fixture, "user.find")
			if fixture.users.updatedStatus != valueobject.StatusUnknown {
				t.Fatalf(
					"PostgreSQL status changed to %s after rejected transition",
					fixture.users.updatedStatus,
				)
			}
			if fixture.sessions.blockCalls != 0 || fixture.sessions.unblockCalls != 0 {
				t.Fatalf(
					"Redis access-state calls: block=%d unblock=%d, want 0/0",
					fixture.sessions.blockCalls,
					fixture.sessions.unblockCalls,
				)
			}
		})
	}
}

func TestReapplyingCurrentStatusIsTrueNoOp(t *testing.T) {
	for _, status := range []valueobject.UserStatus{
		valueobject.StatusInactive,
		valueobject.StatusActive,
		valueobject.StatusSuspended,
	} {
		t.Run(status.String(), func(t *testing.T) {
			fixture := newStatusAccessFixture()
			fixture.users.target.Status = status

			if err := fixture.usecase.ChangeStatus(
				context.Background(),
				statusChangeInput(status),
			); err != nil {
				t.Fatalf("ChangeStatus() error = %v", err)
			}
			assertStatusAccessCalls(t, fixture, "user.find")
			if fixture.users.updatedStatus != valueobject.StatusUnknown {
				t.Fatalf(
					"PostgreSQL status changed to %s for same-status request",
					fixture.users.updatedStatus,
				)
			}
			if fixture.sessions.blockCalls != 0 || fixture.sessions.unblockCalls != 0 {
				t.Fatalf(
					"Redis access-state calls: block=%d unblock=%d, want 0/0",
					fixture.sessions.blockCalls,
					fixture.sessions.unblockCalls,
				)
			}
		})
	}
}

func TestDuplicateActivationAndReactivationRequestsAreNoOps(t *testing.T) {
	tests := []struct {
		name       string
		initial    valueobject.UserStatus
		firstCalls []string
	}{
		{
			name:    "activation",
			initial: valueobject.StatusInactive,
			firstCalls: []string{
				"user.find",
				"user.update_status",
			},
		},
		{
			name:    "reactivation",
			initial: valueobject.StatusSuspended,
			firstCalls: []string{
				"user.find",
				"session.unblock",
				"user.update_status",
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			fixture := newStatusAccessFixture()
			fixture.users.target.Status = test.initial

			if err := fixture.usecase.ChangeStatus(
				context.Background(),
				statusChangeInput(valueobject.StatusActive),
			); err != nil {
				t.Fatalf("first ChangeStatus() error = %v", err)
			}
			assertStatusAccessCalls(t, fixture, test.firstCalls...)

			fixture.recorder.calls = nil
			if err := fixture.usecase.ChangeStatus(
				context.Background(),
				statusChangeInput(valueobject.StatusActive),
			); err != nil {
				t.Fatalf("duplicate ChangeStatus() error = %v", err)
			}
			assertStatusAccessCalls(t, fixture, "user.find")
		})
	}
}

func statusChangeInput(status valueobject.UserStatus) UpdateStatusInput {
	return UpdateStatusInput{
		UserID:    "user-1",
		ActorID:   "admin-1",
		ActorRole: valueobject.RoleAdmin,
		Status:    status,
	}
}

type statusAccessFixture struct {
	recorder *statusAccessRecorder
	users    *statusAccessUserRepository
	sessions *statusAccessSessionRepository
	usecase  UseCase
}

func newStatusAccessFixture() *statusAccessFixture {
	recorder := &statusAccessRecorder{}
	users := &statusAccessUserRepository{
		recorder: recorder,
		target: &entity.User{
			ID:     "user-1",
			Role:   valueobject.RoleClient,
			Status: valueobject.StatusActive,
		},
	}
	sessions := &statusAccessSessionRepository{recorder: recorder}
	usecase := NewUsecase(
		users,
		nil,
		discardUserLogger{},
		sessions,
		nil,
	)
	return &statusAccessFixture{
		recorder: recorder,
		users:    users,
		sessions: sessions,
		usecase:  usecase,
	}
}

func assertStatusAccessCalls(
	t *testing.T,
	fixture *statusAccessFixture,
	want ...string,
) {
	t.Helper()
	if !reflect.DeepEqual(fixture.recorder.calls, want) {
		t.Fatalf("call order = %v, want %v", fixture.recorder.calls, want)
	}
}

type statusAccessRecorder struct {
	calls []string
}

func (r *statusAccessRecorder) record(call string) {
	r.calls = append(r.calls, call)
}

type statusAccessUserRepository struct {
	repository.UserRepository
	recorder      *statusAccessRecorder
	target        *entity.User
	updateErr     error
	updateResult  *repository.UserStatusUpdateResult
	updatedStatus valueobject.UserStatus
}

func (r *statusAccessUserRepository) FindByID(
	context.Context,
	string,
) (*entity.User, error) {
	r.recorder.record("user.find")
	return r.target, nil
}

func (r *statusAccessUserRepository) UpdateStatus(
	_ context.Context,
	_ string,
	_ valueobject.UserStatus,
	requested valueobject.UserStatus,
) (repository.UserStatusUpdateResult, error) {
	r.recorder.record("user.update_status")
	if r.updateErr != nil {
		return repository.UserStatusUpdateResult{}, r.updateErr
	}
	if r.updateResult != nil {
		return *r.updateResult, nil
	}
	r.updatedStatus = requested
	r.target.Status = requested
	return repository.UserStatusUpdateResult{
		Outcome:       repository.UserStatusUpdateApplied,
		CurrentStatus: requested,
	}, nil
}

type statusAccessSessionRepository struct {
	repository.SessionRepository
	recorder           *statusAccessRecorder
	blockErr           error
	unblockErr         error
	blockCalls         int
	unblockCalls       int
	blockDeadline      time.Time
	blockHasDeadline   bool
	unblockDeadline    time.Time
	unblockHasDeadline bool
}

func (r *statusAccessSessionRepository) BlockAndDeleteAllByUser(
	ctx context.Context,
	_ string,
) error {
	r.recorder.record("session.block_delete_all")
	r.blockCalls++
	r.blockDeadline, r.blockHasDeadline = ctx.Deadline()
	return r.blockErr
}

func (r *statusAccessSessionRepository) UnblockUser(
	ctx context.Context,
	_ string,
) error {
	r.recorder.record("session.unblock")
	r.unblockCalls++
	r.unblockDeadline, r.unblockHasDeadline = ctx.Deadline()
	return r.unblockErr
}
