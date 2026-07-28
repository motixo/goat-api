package user

import (
	"context"
	stdErrors "errors"
	"fmt"
	"sync"
	"testing"

	"github.com/motixo/goat-api/internal/domain/entity"
	domainErrors "github.com/motixo/goat-api/internal/domain/errors"
	"github.com/motixo/goat-api/internal/domain/repository"
	"github.com/motixo/goat-api/internal/domain/valueobject"
	userrolechange "github.com/motixo/goat-api/internal/usecase/user/rolechange"
)

func TestChangeRoleUsesCredentialFreeSnapshotAndFocusedWriter(t *testing.T) {
	reader := &roleChangeSnapshotReader{snapshot: UserStatusSnapshot{
		ID:     "11111111-1111-4111-8111-111111111111",
		Role:   valueobject.RoleClient,
		Status: valueobject.StatusActive,
	}}
	writer := &roleChangeWriter{}
	usecase := newRoleChangeUsecase(reader, writer)

	err := usecase.ChangeRole(context.Background(), UpdateRoleInput{
		UserID:    reader.snapshot.ID,
		ActorID:   "22222222-2222-4222-8222-222222222222",
		ActorRole: valueobject.RoleAdmin,
		Role:      valueobject.RoleOperator,
	})
	if err != nil {
		t.Fatalf("ChangeRole() error = %v", err)
	}
	if reader.calls != 1 || reader.gotID != reader.snapshot.ID {
		t.Fatalf(
			"snapshot calls/id = %d/%q, want 1/%q",
			reader.calls,
			reader.gotID,
			reader.snapshot.ID,
		)
	}
	if writer.calls != 1 {
		t.Fatalf("UpdateRole calls = %d, want 1", writer.calls)
	}
	wantCommand := UserRoleUpdateCommand{
		UserID:        reader.snapshot.ID,
		ExpectedRole:  valueobject.RoleClient,
		RequestedRole: valueobject.RoleOperator,
	}
	if writer.command != wantCommand {
		t.Fatalf("UpdateRole command = %#v, want %#v", writer.command, wantCommand)
	}
}

func TestChangeRoleDoesNotOverwriteRoleChangedAfterSnapshot(t *testing.T) {
	users := &staleRoleChangeWriter{
		currentRole:     valueobject.RoleClient,
		roleBeforeWrite: valueobject.RoleAdmin,
	}
	usecase := newRoleChangeUsecase(users, users)

	err := usecase.ChangeRole(context.Background(), UpdateRoleInput{
		UserID:    "target-user",
		ActorID:   "operator-user",
		ActorRole: valueobject.RoleOperator,
		Role:      valueobject.RoleClient,
	})

	if !stdErrors.Is(err, userrolechange.ErrConcurrentRoleChange) {
		t.Fatalf("ChangeRole() error = %v, want ErrConcurrentRoleChange", err)
	}
	if users.currentRole != valueobject.RoleAdmin {
		t.Fatalf(
			"authoritative role = %s, want concurrent admin role preserved",
			users.currentRole,
		)
	}
	if users.updateCalls != 1 {
		t.Fatalf("UpdateRole calls = %d, want 1 without retry", users.updateCalls)
	}
}

func TestChangeRoleCompareAndSetResultSemantics(t *testing.T) {
	tests := []struct {
		name       string
		result     UserRoleUpdateResult
		wantErr    error
		wantAnyErr bool
	}{
		{
			name: "destination already reached",
			result: UserRoleUpdateResult{
				Outcome:     UserRoleUpdateAlreadyApplied,
				CurrentRole: valueobject.RoleOperator,
			},
		},
		{
			name: "user deleted between read and update",
			result: UserRoleUpdateResult{
				Outcome: UserRoleUpdateNotFound,
			},
			wantErr: domainErrors.ErrUserNotFound,
		},
		{
			name: "conflicting current role",
			result: UserRoleUpdateResult{
				Outcome:     UserRoleUpdateConflict,
				CurrentRole: valueobject.RoleAdmin,
			},
			wantErr: userrolechange.ErrConcurrentRoleChange,
		},
		{
			name: "invalid applied result",
			result: UserRoleUpdateResult{
				Outcome:     UserRoleUpdateApplied,
				CurrentRole: valueobject.RoleAdmin,
			},
			wantAnyErr: true,
		},
		{
			name: "invalid idempotent result",
			result: UserRoleUpdateResult{
				Outcome:     UserRoleUpdateAlreadyApplied,
				CurrentRole: valueobject.RoleAdmin,
			},
			wantAnyErr: true,
		},
		{
			name: "conflict still reports expected role",
			result: UserRoleUpdateResult{
				Outcome:     UserRoleUpdateConflict,
				CurrentRole: valueobject.RoleClient,
			},
			wantAnyErr: true,
		},
		{
			name: "unknown outcome",
			result: UserRoleUpdateResult{
				Outcome: UserRoleUpdateUnknown,
			},
			wantAnyErr: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			reader := &roleChangeSnapshotReader{snapshot: UserStatusSnapshot{
				ID:     "target-user",
				Role:   valueobject.RoleClient,
				Status: valueobject.StatusActive,
			}}
			writer := &roleChangeWriter{result: &test.result}
			usecase := newRoleChangeUsecase(reader, writer)

			err := usecase.ChangeRole(context.Background(), validRoleChangeInput())
			if test.wantErr != nil {
				if !stdErrors.Is(err, test.wantErr) {
					t.Fatalf("ChangeRole() error = %v, want %v", err, test.wantErr)
				}
			} else if test.wantAnyErr {
				if err == nil ||
					stdErrors.Is(err, domainErrors.ErrUserNotFound) ||
					stdErrors.Is(err, userrolechange.ErrConcurrentRoleChange) {
					t.Fatalf("ChangeRole() error = %v, want safe internal failure", err)
				}
			} else if err != nil {
				t.Fatalf("ChangeRole() error = %v", err)
			}
			if writer.calls != 1 {
				t.Fatalf("UpdateRole calls = %d, want 1", writer.calls)
			}
		})
	}
}

func TestConcurrentConflictingRoleChangesCommitAtMostOnce(t *testing.T) {
	readsReady := &sync.WaitGroup{}
	readsReady.Add(2)
	releaseReads := make(chan struct{})
	users := &concurrentRoleChangeRepository{
		role:         valueobject.RoleClient,
		readsReady:   readsReady,
		releaseReads: releaseReads,
	}
	usecase := newRoleChangeUsecase(users, users)

	start := make(chan struct{})
	requestedRoles := []valueobject.UserRole{
		valueobject.RoleOperator,
		valueobject.RoleAdmin,
	}
	results := make(chan error, len(requestedRoles))
	for _, requestedRole := range requestedRoles {
		requestedRole := requestedRole
		go func() {
			<-start
			results <- usecase.ChangeRole(context.Background(), UpdateRoleInput{
				UserID:    "target-user",
				ActorID:   "admin-user",
				ActorRole: valueobject.RoleAdmin,
				Role:      requestedRole,
			})
		}()
	}
	close(start)
	readsReady.Wait()
	close(releaseReads)

	successes := 0
	conflicts := 0
	for range requestedRoles {
		err := <-results
		switch {
		case err == nil:
			successes++
		case stdErrors.Is(err, userrolechange.ErrConcurrentRoleChange):
			conflicts++
		default:
			t.Fatalf("concurrent ChangeRole() error = %v", err)
		}
	}
	if successes != 1 || conflicts != 1 {
		t.Fatalf("concurrent outcomes = success %d/conflict %d, want 1/1", successes, conflicts)
	}
	if users.updateCallCount() != 2 {
		t.Fatalf("UpdateRole calls = %d, want 2 without retry", users.updateCallCount())
	}
	if role := users.currentRole(); role != valueobject.RoleOperator && role != valueobject.RoleAdmin {
		t.Fatalf("authoritative role = %s, want committed operator or admin role", role)
	}
}

func TestChangeRoleEnforcesTargetRoleHierarchy(t *testing.T) {
	tests := []struct {
		name       string
		actorRole  valueobject.UserRole
		targetRole valueobject.UserRole
		wantErr    error
	}{
		{name: "administrator may modify administrator", actorRole: valueobject.RoleAdmin, targetRole: valueobject.RoleAdmin},
		{name: "administrator may modify operator", actorRole: valueobject.RoleAdmin, targetRole: valueobject.RoleOperator},
		{name: "administrator may modify client", actorRole: valueobject.RoleAdmin, targetRole: valueobject.RoleClient},
		{name: "operator may modify client", actorRole: valueobject.RoleOperator, targetRole: valueobject.RoleClient},
		{name: "operator may not modify operator", actorRole: valueobject.RoleOperator, targetRole: valueobject.RoleOperator, wantErr: domainErrors.ErrForbidden},
		{name: "operator may not modify administrator", actorRole: valueobject.RoleOperator, targetRole: valueobject.RoleAdmin, wantErr: domainErrors.ErrForbidden},
		{name: "client may not modify client", actorRole: valueobject.RoleClient, targetRole: valueobject.RoleClient, wantErr: domainErrors.ErrForbidden},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			reader := &roleChangeSnapshotReader{snapshot: UserStatusSnapshot{
				ID:     "target-user",
				Role:   test.targetRole,
				Status: valueobject.StatusActive,
			}}
			writer := &roleChangeWriter{}
			usecase := newRoleChangeUsecase(reader, writer)

			err := usecase.ChangeRole(context.Background(), UpdateRoleInput{
				UserID:    "target-user",
				ActorID:   "actor-user",
				ActorRole: test.actorRole,
				Role:      valueobject.RoleClient,
			})
			if !stdErrors.Is(err, test.wantErr) {
				t.Fatalf("ChangeRole() error = %v, want %v", err, test.wantErr)
			}
			wantWriterCalls := 1
			if test.wantErr != nil {
				wantWriterCalls = 0
			}
			if writer.calls != wantWriterCalls {
				t.Fatalf("UpdateRole calls = %d, want %d", writer.calls, wantWriterCalls)
			}
		})
	}
}

func TestChangeRolePreservesAssignableAndSameRoleBehavior(t *testing.T) {
	for _, requestedRole := range valueobject.AllRoles() {
		t.Run(requestedRole.String(), func(t *testing.T) {
			reader := &roleChangeSnapshotReader{snapshot: UserStatusSnapshot{
				ID:     "admin-user",
				Role:   requestedRole,
				Status: valueobject.StatusActive,
			}}
			writer := &roleChangeWriter{}
			usecase := newRoleChangeUsecase(reader, writer)

			err := usecase.ChangeRole(context.Background(), UpdateRoleInput{
				UserID:    "admin-user",
				ActorID:   "admin-user",
				ActorRole: valueobject.RoleAdmin,
				Role:      requestedRole,
			})
			if err != nil {
				t.Fatalf("ChangeRole() error = %v", err)
			}
			if writer.calls != 1 || writer.command.RequestedRole != requestedRole {
				t.Fatalf(
					"UpdateRole calls/role = %d/%s, want 1/%s",
					writer.calls,
					writer.command.RequestedRole,
					requestedRole,
				)
			}
		})
	}
}

func TestChangeRoleEnforcesAssignmentHierarchy(t *testing.T) {
	for _, test := range []struct {
		name          string
		actorRole     valueobject.UserRole
		requestedRole valueobject.UserRole
		wantErr       error
	}{
		{name: "administrator may assign administrator", actorRole: valueobject.RoleAdmin, requestedRole: valueobject.RoleAdmin},
		{name: "administrator may assign operator", actorRole: valueobject.RoleAdmin, requestedRole: valueobject.RoleOperator},
		{name: "administrator may assign client", actorRole: valueobject.RoleAdmin, requestedRole: valueobject.RoleClient},
		{name: "operator may assign client", actorRole: valueobject.RoleOperator, requestedRole: valueobject.RoleClient},
		{name: "operator may not assign operator", actorRole: valueobject.RoleOperator, requestedRole: valueobject.RoleOperator, wantErr: domainErrors.ErrForbidden},
		{name: "operator may not assign administrator", actorRole: valueobject.RoleOperator, requestedRole: valueobject.RoleAdmin, wantErr: domainErrors.ErrForbidden},
	} {
		t.Run(test.name, func(t *testing.T) {
			reader := &roleChangeSnapshotReader{snapshot: UserStatusSnapshot{
				ID:     "target-client",
				Role:   valueobject.RoleClient,
				Status: valueobject.StatusActive,
			}}
			writer := &roleChangeWriter{}
			usecase := newRoleChangeUsecase(reader, writer)

			err := usecase.ChangeRole(context.Background(), UpdateRoleInput{
				UserID:    "target-client",
				ActorID:   "actor-user",
				ActorRole: test.actorRole,
				Role:      test.requestedRole,
			})
			if !stdErrors.Is(err, test.wantErr) {
				t.Fatalf("ChangeRole() error = %v, want %v", err, test.wantErr)
			}
			wantCalls := 1
			if test.wantErr != nil {
				wantCalls = 0
			}
			if writer.calls != wantCalls {
				t.Fatalf("UpdateRole calls = %d, want %d", writer.calls, wantCalls)
			}
		})
	}
}

func TestChangeRoleSelfChangeUsesTheSameRoleHierarchy(t *testing.T) {
	for _, test := range []struct {
		name      string
		actorRole valueobject.UserRole
		wantErr   error
	}{
		{name: "administrator self change remains allowed", actorRole: valueobject.RoleAdmin},
		{name: "operator self change remains forbidden", actorRole: valueobject.RoleOperator, wantErr: domainErrors.ErrForbidden},
		{name: "client self change remains forbidden", actorRole: valueobject.RoleClient, wantErr: domainErrors.ErrForbidden},
	} {
		t.Run(test.name, func(t *testing.T) {
			reader := &roleChangeSnapshotReader{snapshot: UserStatusSnapshot{
				ID:     "actor-user",
				Role:   test.actorRole,
				Status: valueobject.StatusActive,
			}}
			writer := &roleChangeWriter{}
			usecase := newRoleChangeUsecase(reader, writer)

			err := usecase.ChangeRole(context.Background(), UpdateRoleInput{
				UserID:    "actor-user",
				ActorID:   "actor-user",
				ActorRole: test.actorRole,
				Role:      valueobject.RoleClient,
			})
			if !stdErrors.Is(err, test.wantErr) {
				t.Fatalf("ChangeRole() error = %v, want %v", err, test.wantErr)
			}
			wantCalls := 1
			if test.wantErr != nil {
				wantCalls = 0
			}
			if writer.calls != wantCalls {
				t.Fatalf("UpdateRole calls = %d, want %d", writer.calls, wantCalls)
			}
		})
	}
}

func TestChangeRoleRejectsInvalidRequestedRoleBeforeLookupOrMutation(t *testing.T) {
	for _, requestedRole := range []valueobject.UserRole{
		valueobject.RoleUnknown,
		valueobject.UserRole(255),
	} {
		reader := &roleChangeSnapshotReader{}
		writer := &roleChangeWriter{}
		usecase := newRoleChangeUsecase(reader, writer)

		err := usecase.ChangeRole(context.Background(), UpdateRoleInput{
			UserID:    "target-user",
			ActorID:   "admin-user",
			ActorRole: valueobject.RoleAdmin,
			Role:      requestedRole,
		})
		if !stdErrors.Is(err, domainErrors.ErrBadRequest) {
			t.Fatalf("ChangeRole(role=%d) error = %v, want ErrBadRequest", requestedRole, err)
		}
		if reader.calls != 0 || writer.calls != 0 {
			t.Fatalf(
				"role=%d snapshot/write calls = %d/%d, want 0/0",
				requestedRole,
				reader.calls,
				writer.calls,
			)
		}
	}
}

func TestChangeRolePropagatesLookupAndWriterErrors(t *testing.T) {
	t.Run("target lookup", func(t *testing.T) {
		lookupErr := fmt.Errorf("read target: %w", domainErrors.ErrUserNotFound)
		reader := &roleChangeSnapshotReader{err: lookupErr}
		writer := &roleChangeWriter{}
		usecase := newRoleChangeUsecase(reader, writer)

		err := usecase.ChangeRole(context.Background(), validRoleChangeInput())
		if !stdErrors.Is(err, domainErrors.ErrUserNotFound) {
			t.Fatalf("ChangeRole() error = %v, want ErrUserNotFound", err)
		}
		if writer.calls != 0 {
			t.Fatalf("UpdateRole calls = %d, want 0", writer.calls)
		}
	})

	t.Run("role update", func(t *testing.T) {
		writeErr := stdErrors.New("postgres role update failed")
		reader := &roleChangeSnapshotReader{snapshot: UserStatusSnapshot{
			ID:     "target-user",
			Role:   valueobject.RoleClient,
			Status: valueobject.StatusActive,
		}}
		writer := &roleChangeWriter{err: writeErr}
		usecase := newRoleChangeUsecase(reader, writer)

		err := usecase.ChangeRole(context.Background(), validRoleChangeInput())
		if !stdErrors.Is(err, writeErr) {
			t.Fatalf("ChangeRole() error = %v, want writer failure", err)
		}
	})
}

func TestChangeRoleRejectsInvalidAuthoritativeTargetRole(t *testing.T) {
	reader := &roleChangeSnapshotReader{snapshot: UserStatusSnapshot{
		ID:     "target-user",
		Role:   valueobject.UserRole(255),
		Status: valueobject.StatusActive,
	}}
	writer := &roleChangeWriter{}
	usecase := newRoleChangeUsecase(reader, writer)

	err := usecase.ChangeRole(context.Background(), validRoleChangeInput())
	if err == nil || stdErrors.Is(err, domainErrors.ErrForbidden) {
		t.Fatalf("ChangeRole() error = %v, want safe internal failure", err)
	}
	if writer.calls != 0 {
		t.Fatalf("UpdateRole calls = %d, want 0", writer.calls)
	}
}

func validRoleChangeInput() UpdateRoleInput {
	return UpdateRoleInput{
		UserID:    "target-user",
		ActorID:   "admin-user",
		ActorRole: valueobject.RoleAdmin,
		Role:      valueobject.RoleOperator,
	}
}

func newRoleChangeUsecase(
	reader UserStatusSnapshotReader,
	writer UserRoleUpdateWriter,
) UseCase {
	return NewUsecase(Dependencies{
		UserRepository: &panicOnRoleChangeFullUserRepository{},
		StatusReader:   reader,
		RoleWriter:     writer,
		Logger:         discardUserLogger{},
	})
}

type panicOnRoleChangeFullUserRepository struct {
	repository.UserRepository
}

func (*panicOnRoleChangeFullUserRepository) FindByID(
	context.Context,
	string,
) (*entity.User, error) {
	panic("role change must not load a credential-bearing User aggregate")
}

type roleChangeSnapshotReader struct {
	snapshot UserStatusSnapshot
	err      error
	calls    int
	gotID    string
}

func (r *roleChangeSnapshotReader) FindStatusSnapshotByID(
	_ context.Context,
	id string,
) (UserStatusSnapshot, error) {
	r.calls++
	r.gotID = id
	return r.snapshot, r.err
}

type roleChangeWriter struct {
	command UserRoleUpdateCommand
	result  *UserRoleUpdateResult
	err     error
	calls   int
}

func (w *roleChangeWriter) UpdateRole(
	_ context.Context,
	command UserRoleUpdateCommand,
) (UserRoleUpdateResult, error) {
	w.calls++
	w.command = command
	if w.result != nil {
		return *w.result, w.err
	}
	return UserRoleUpdateResult{
		Outcome:     UserRoleUpdateApplied,
		CurrentRole: command.RequestedRole,
	}, w.err
}

type staleRoleChangeWriter struct {
	currentRole     valueobject.UserRole
	roleBeforeWrite valueobject.UserRole
	updateCalls     int
}

type concurrentRoleChangeRepository struct {
	mu           sync.Mutex
	role         valueobject.UserRole
	updateCalls  int
	readsReady   *sync.WaitGroup
	releaseReads <-chan struct{}
}

func (r *concurrentRoleChangeRepository) FindStatusSnapshotByID(
	context.Context,
	string,
) (UserStatusSnapshot, error) {
	r.mu.Lock()
	role := r.role
	r.mu.Unlock()

	r.readsReady.Done()
	<-r.releaseReads
	return UserStatusSnapshot{
		ID:     "target-user",
		Role:   role,
		Status: valueobject.StatusActive,
	}, nil
}

func (r *concurrentRoleChangeRepository) UpdateRole(
	_ context.Context,
	command UserRoleUpdateCommand,
) (UserRoleUpdateResult, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.updateCalls++
	switch r.role {
	case command.ExpectedRole:
		r.role = command.RequestedRole
		return UserRoleUpdateResult{
			Outcome:     UserRoleUpdateApplied,
			CurrentRole: command.RequestedRole,
		}, nil
	case command.RequestedRole:
		return UserRoleUpdateResult{
			Outcome:     UserRoleUpdateAlreadyApplied,
			CurrentRole: r.role,
		}, nil
	default:
		return UserRoleUpdateResult{
			Outcome:     UserRoleUpdateConflict,
			CurrentRole: r.role,
		}, nil
	}
}

func (r *concurrentRoleChangeRepository) currentRole() valueobject.UserRole {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.role
}

func (r *concurrentRoleChangeRepository) updateCallCount() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.updateCalls
}

func (w *staleRoleChangeWriter) FindStatusSnapshotByID(
	context.Context,
	string,
) (UserStatusSnapshot, error) {
	return UserStatusSnapshot{
		ID:     "target-user",
		Role:   w.currentRole,
		Status: valueobject.StatusActive,
	}, nil
}

func (w *staleRoleChangeWriter) UpdateRole(
	_ context.Context,
	command UserRoleUpdateCommand,
) (UserRoleUpdateResult, error) {
	w.updateCalls++
	w.currentRole = w.roleBeforeWrite
	if w.currentRole == command.ExpectedRole {
		w.currentRole = command.RequestedRole
		return UserRoleUpdateResult{
			Outcome:     UserRoleUpdateApplied,
			CurrentRole: command.RequestedRole,
		}, nil
	}
	if w.currentRole == command.RequestedRole {
		return UserRoleUpdateResult{
			Outcome:     UserRoleUpdateAlreadyApplied,
			CurrentRole: w.currentRole,
		}, nil
	}
	return UserRoleUpdateResult{
		Outcome:     UserRoleUpdateConflict,
		CurrentRole: w.currentRole,
	}, nil
}
