package user

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/motixo/goat-api/internal/domain/entity"
	domainErrors "github.com/motixo/goat-api/internal/domain/errors"
	"github.com/motixo/goat-api/internal/domain/repository"
	"github.com/motixo/goat-api/internal/domain/valueobject"
)

func TestChangeStatusUsesCredentialFreeStatusSnapshotReader(t *testing.T) {
	users := &statusSnapshotRegressionRepository{}
	reader := &recordingUserStatusSnapshotReader{
		snapshot: UserStatusSnapshot{
			ID:     "11111111-1111-4111-8111-111111111111",
			Role:   valueobject.RoleClient,
			Status: valueobject.StatusInactive,
		},
	}
	usecase := NewUsecase(Dependencies{
		UserRepository: users,
		StatusReader:   reader,
		Logger:         discardUserLogger{},
	})

	err := usecase.ChangeStatus(context.Background(), UpdateStatusInput{
		UserID:    reader.snapshot.ID,
		ActorID:   "22222222-2222-4222-8222-222222222222",
		ActorRole: valueobject.RoleAdmin,
		Status:    valueobject.StatusActive,
	})
	if err != nil {
		t.Fatalf("ChangeStatus() error = %v", err)
	}
	if reader.calls != 1 || reader.gotID != reader.snapshot.ID {
		t.Fatalf(
			"FindStatusSnapshotByID calls/id = %d/%q, want 1/%q",
			reader.calls,
			reader.gotID,
			reader.snapshot.ID,
		)
	}
	if users.updateCalls != 1 ||
		users.expected != valueobject.StatusInactive ||
		users.requested != valueobject.StatusActive {
		t.Fatalf(
			"UpdateStatus calls/expected/requested = %d/%s/%s, want 1/inactive/active",
			users.updateCalls,
			users.expected,
			users.requested,
		)
	}
}

func TestChangeStatusPropagatesSnapshotNotFoundWithoutMutation(t *testing.T) {
	users := &statusSnapshotRegressionRepository{}
	reader := &recordingUserStatusSnapshotReader{
		err: fmt.Errorf("read authoritative status snapshot: %w", domainErrors.ErrUserNotFound),
	}
	usecase := NewUsecase(Dependencies{
		UserRepository: users,
		StatusReader:   reader,
		Logger:         discardUserLogger{},
	})

	err := usecase.ChangeStatus(context.Background(), UpdateStatusInput{
		UserID:    "33333333-3333-4333-8333-333333333333",
		ActorID:   "22222222-2222-4222-8222-222222222222",
		ActorRole: valueobject.RoleAdmin,
		Status:    valueobject.StatusActive,
	})
	if !errors.Is(err, domainErrors.ErrUserNotFound) {
		t.Fatalf("ChangeStatus() error = %v, want ErrUserNotFound", err)
	}
	if users.updateCalls != 0 {
		t.Fatalf("UpdateStatus calls = %d, want 0", users.updateCalls)
	}
}

func TestChangeStatusUsesSnapshotRoleForTargetAuthorization(t *testing.T) {
	users := &statusSnapshotRegressionRepository{}
	reader := &recordingUserStatusSnapshotReader{
		snapshot: UserStatusSnapshot{
			ID:     "11111111-1111-4111-8111-111111111111",
			Role:   valueobject.RoleAdmin,
			Status: valueobject.StatusActive,
		},
	}
	usecase := NewUsecase(Dependencies{
		UserRepository: users,
		StatusReader:   reader,
		Logger:         discardUserLogger{},
	})

	err := usecase.ChangeStatus(context.Background(), UpdateStatusInput{
		UserID:    reader.snapshot.ID,
		ActorID:   "22222222-2222-4222-8222-222222222222",
		ActorRole: valueobject.RoleOperator,
		Status:    valueobject.StatusSuspended,
	})
	if !errors.Is(err, domainErrors.ErrForbidden) {
		t.Fatalf("ChangeStatus() error = %v, want ErrForbidden", err)
	}
	if users.updateCalls != 0 {
		t.Fatalf("UpdateStatus calls = %d, want 0", users.updateCalls)
	}
}

type recordingUserStatusSnapshotReader struct {
	snapshot UserStatusSnapshot
	err      error
	calls    int
	gotID    string
}

func (r *recordingUserStatusSnapshotReader) FindStatusSnapshotByID(
	_ context.Context,
	id string,
) (UserStatusSnapshot, error) {
	r.calls++
	r.gotID = id
	return r.snapshot, r.err
}

type statusSnapshotRegressionRepository struct {
	repository.UserRepository
	updateCalls int
	expected    valueobject.UserStatus
	requested   valueobject.UserStatus
}

func (*statusSnapshotRegressionRepository) FindByID(
	context.Context,
	string,
) (*entity.User, error) {
	panic("status transition loaded the credential-bearing User aggregate")
}

func (r *statusSnapshotRegressionRepository) UpdateStatus(
	_ context.Context,
	_ string,
	expected valueobject.UserStatus,
	requested valueobject.UserStatus,
) (repository.UserStatusUpdateResult, error) {
	r.updateCalls++
	r.expected = expected
	r.requested = requested
	return repository.UserStatusUpdateResult{
		Outcome:       repository.UserStatusUpdateApplied,
		CurrentStatus: requested,
	}, nil
}
