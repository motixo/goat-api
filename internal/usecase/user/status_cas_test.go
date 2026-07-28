package user

import (
	"context"
	"errors"
	"sync"
	"testing"

	domainErrors "github.com/motixo/goat-api/internal/domain/errors"
	"github.com/motixo/goat-api/internal/domain/repository"
	"github.com/motixo/goat-api/internal/domain/valueobject"
)

func TestChangeStatusDoesNotOverwriteStatusCommittedAfterItsRead(t *testing.T) {
	users := &staleStatusUserRepository{
		status:               valueobject.StatusInactive,
		statusBeforeCASWrite: valueobject.StatusSuspended,
	}
	usecase := NewUsecase(Dependencies{
		UserRepository: users,
		StatusReader:   users,
		Logger:         discardUserLogger{},
	})

	err := usecase.ChangeStatus(context.Background(), UpdateStatusInput{
		UserID:    "user-1",
		ActorID:   "admin-1",
		ActorRole: valueobject.RoleAdmin,
		Status:    valueobject.StatusActive,
	})

	if !errors.Is(err, domainErrors.ErrInvalidUserStatusTransition) {
		t.Fatalf(
			"ChangeStatus() error = %v, want ErrInvalidUserStatusTransition",
			err,
		)
	}
	if got := users.currentStatus(); got != valueobject.StatusSuspended {
		t.Fatalf("authoritative status = %s, want concurrent suspended status preserved", got)
	}
}

func TestChangeStatusCompareAndSetResultSemantics(t *testing.T) {
	t.Run("destination already reached", func(t *testing.T) {
		fixture := newStatusAccessFixture()
		fixture.users.updateResult = &repository.UserStatusUpdateResult{
			Outcome:       repository.UserStatusUpdateAlreadyApplied,
			CurrentStatus: valueobject.StatusSuspended,
		}

		err := fixture.usecase.ChangeStatus(
			context.Background(),
			statusChangeInput(valueobject.StatusSuspended),
		)

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
	})

	t.Run("conflicting current status", func(t *testing.T) {
		fixture := newStatusAccessFixture()
		fixture.users.updateResult = &repository.UserStatusUpdateResult{
			Outcome:       repository.UserStatusUpdateConflict,
			CurrentStatus: valueobject.StatusInactive,
		}

		err := fixture.usecase.ChangeStatus(
			context.Background(),
			statusChangeInput(valueobject.StatusSuspended),
		)

		if !errors.Is(err, domainErrors.ErrInvalidUserStatusTransition) {
			t.Fatalf(
				"ChangeStatus() error = %v, want ErrInvalidUserStatusTransition",
				err,
			)
		}
		assertStatusAccessCalls(
			t,
			fixture,
			"user.find",
			"session.block_delete_all",
			"user.update_status",
		)
		if fixture.sessions.blockCalls != 1 || fixture.sessions.unblockCalls != 0 {
			t.Fatalf(
				"Redis calls after conflict: block=%d unblock=%d, want 1/0",
				fixture.sessions.blockCalls,
				fixture.sessions.unblockCalls,
			)
		}
	})

	t.Run("user deleted between read and update", func(t *testing.T) {
		fixture := newStatusAccessFixture()
		fixture.users.target.Status = valueobject.StatusInactive
		fixture.users.updateResult = &repository.UserStatusUpdateResult{
			Outcome: repository.UserStatusUpdateNotFound,
		}

		err := fixture.usecase.ChangeStatus(
			context.Background(),
			statusChangeInput(valueobject.StatusActive),
		)

		if !errors.Is(err, domainErrors.ErrUserNotFound) {
			t.Fatalf("ChangeStatus() error = %v, want ErrUserNotFound", err)
		}
		assertStatusAccessCalls(t, fixture, "user.find", "user.update_status")
	})

	t.Run("invalid repository outcome fails closed", func(t *testing.T) {
		fixture := newStatusAccessFixture()
		fixture.users.target.Status = valueobject.StatusInactive
		fixture.users.updateResult = &repository.UserStatusUpdateResult{
			Outcome: repository.UserStatusUpdateUnknown,
		}

		err := fixture.usecase.ChangeStatus(
			context.Background(),
			statusChangeInput(valueobject.StatusActive),
		)

		if err == nil {
			t.Fatal("ChangeStatus() error = nil, want invalid outcome failure")
		}
		if errors.Is(err, domainErrors.ErrUserNotFound) ||
			errors.Is(err, domainErrors.ErrInvalidUserStatusTransition) {
			t.Fatalf("invalid outcome was misclassified: %v", err)
		}
		assertStatusAccessCalls(t, fixture, "user.find", "user.update_status")
	})
}

func TestConcurrentIdenticalStatusChangesCommitOnce(t *testing.T) {
	tests := []struct {
		name         string
		initial      valueobject.UserStatus
		requested    valueobject.UserStatus
		wantBlocks   int
		wantUnblocks int
	}{
		{
			name:      "activation",
			initial:   valueobject.StatusInactive,
			requested: valueobject.StatusActive,
		},
		{
			name:       "suspension",
			initial:    valueobject.StatusActive,
			requested:  valueobject.StatusSuspended,
			wantBlocks: 2,
		},
		{
			name:         "reactivation",
			initial:      valueobject.StatusSuspended,
			requested:    valueobject.StatusActive,
			wantUnblocks: 2,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			readsReady := &sync.WaitGroup{}
			readsReady.Add(2)
			releaseReads := make(chan struct{})
			users := &concurrentStatusUserRepository{
				status:       test.initial,
				readsReady:   readsReady,
				releaseReads: releaseReads,
			}
			sessions := &concurrentStatusSessionRepository{}
			usecase := NewUsecase(Dependencies{
				UserRepository:    users,
				StatusReader:      users,
				Logger:            discardUserLogger{},
				SessionRepository: sessions,
			})

			start := make(chan struct{})
			results := make(chan error, 2)
			for range 2 {
				go func() {
					<-start
					results <- usecase.ChangeStatus(
						context.Background(),
						UpdateStatusInput{
							UserID:    "user-1",
							ActorID:   "admin-1",
							ActorRole: valueobject.RoleAdmin,
							Status:    test.requested,
						},
					)
				}()
			}
			close(start)
			readsReady.Wait()
			close(releaseReads)

			for range 2 {
				if err := <-results; err != nil {
					t.Fatalf("concurrent ChangeStatus() error = %v", err)
				}
			}

			if got := users.currentStatus(); got != test.requested {
				t.Fatalf("authoritative status = %s, want %s", got, test.requested)
			}
			if got := users.updateCallCount(); got != 2 {
				t.Fatalf("compare-and-set calls = %d, want 2", got)
			}
			blocks, unblocks := sessions.callCounts()
			if blocks != test.wantBlocks || unblocks != test.wantUnblocks {
				t.Fatalf(
					"Redis calls = block %d/unblock %d, want %d/%d",
					blocks,
					unblocks,
					test.wantBlocks,
					test.wantUnblocks,
				)
			}
		})
	}
}

type staleStatusUserRepository struct {
	repository.UserRepository

	mu                   sync.Mutex
	status               valueobject.UserStatus
	statusBeforeCASWrite valueobject.UserStatus
}

func (r *staleStatusUserRepository) FindStatusSnapshotByID(
	context.Context,
	string,
) (UserStatusSnapshot, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	return UserStatusSnapshot{
		ID:     "user-1",
		Role:   valueobject.RoleClient,
		Status: r.status,
	}, nil
}

func (r *staleStatusUserRepository) UpdateStatus(
	_ context.Context,
	_ string,
	expected valueobject.UserStatus,
	requested valueobject.UserStatus,
) (repository.UserStatusUpdateResult, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.status = r.statusBeforeCASWrite
	switch r.status {
	case expected:
		r.status = requested
		return repository.UserStatusUpdateResult{
			Outcome:       repository.UserStatusUpdateApplied,
			CurrentStatus: requested,
		}, nil
	case requested:
		return repository.UserStatusUpdateResult{
			Outcome:       repository.UserStatusUpdateAlreadyApplied,
			CurrentStatus: requested,
		}, nil
	default:
		return repository.UserStatusUpdateResult{
			Outcome:       repository.UserStatusUpdateConflict,
			CurrentStatus: r.status,
		}, nil
	}
}

func (r *staleStatusUserRepository) currentStatus() valueobject.UserStatus {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.status
}

type concurrentStatusUserRepository struct {
	repository.UserRepository

	mu           sync.Mutex
	status       valueobject.UserStatus
	updateCalls  int
	readsReady   *sync.WaitGroup
	releaseReads <-chan struct{}
}

func (r *concurrentStatusUserRepository) FindStatusSnapshotByID(
	context.Context,
	string,
) (UserStatusSnapshot, error) {
	r.mu.Lock()
	status := r.status
	r.mu.Unlock()

	r.readsReady.Done()
	<-r.releaseReads
	return UserStatusSnapshot{
		ID:     "user-1",
		Role:   valueobject.RoleClient,
		Status: status,
	}, nil
}

func (r *concurrentStatusUserRepository) UpdateStatus(
	_ context.Context,
	_ string,
	expected valueobject.UserStatus,
	requested valueobject.UserStatus,
) (repository.UserStatusUpdateResult, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.updateCalls++
	switch r.status {
	case expected:
		r.status = requested
		return repository.UserStatusUpdateResult{
			Outcome:       repository.UserStatusUpdateApplied,
			CurrentStatus: requested,
		}, nil
	case requested:
		return repository.UserStatusUpdateResult{
			Outcome:       repository.UserStatusUpdateAlreadyApplied,
			CurrentStatus: requested,
		}, nil
	default:
		return repository.UserStatusUpdateResult{
			Outcome:       repository.UserStatusUpdateConflict,
			CurrentStatus: r.status,
		}, nil
	}
}

func (r *concurrentStatusUserRepository) currentStatus() valueobject.UserStatus {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.status
}

func (r *concurrentStatusUserRepository) updateCallCount() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.updateCalls
}

type concurrentStatusSessionRepository struct {
	repository.SessionRepository

	mu           sync.Mutex
	blockCalls   int
	unblockCalls int
}

func (r *concurrentStatusSessionRepository) BlockAndDeleteAllByUser(
	context.Context,
	string,
) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.blockCalls++
	return nil
}

func (r *concurrentStatusSessionRepository) UnblockUser(
	context.Context,
	string,
) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.unblockCalls++
	return nil
}

func (r *concurrentStatusSessionRepository) callCounts() (int, int) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.blockCalls, r.unblockCalls
}
