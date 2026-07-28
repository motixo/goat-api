package user

import (
	"context"
	"errors"
	"testing"

	"github.com/motixo/goat-api/internal/domain/entity"
	domainErrors "github.com/motixo/goat-api/internal/domain/errors"
	"github.com/motixo/goat-api/internal/domain/repository"
)

func TestGenericUpdateNeverReadsStatusSnapshot(t *testing.T) {
	users := &panicOnGenericUpdateFullUserRepository{}
	usecase := NewUsecase(Dependencies{
		UserRepository: users,
		StatusReader:   &panicOnGenericUpdateStatusReader{},
		UpdateWriter:   users,
		PasswordHasher: &statusStateMachinePasswordHasher{},
		Logger:         discardUserLogger{},
	})

	err := usecase.UpdateUser(context.Background(), UpdateInput{
		UserID:   "user-1",
		Email:    "updated@example.com",
		Password: "Password1!",
	})

	if err != nil {
		t.Fatalf("UpdateUser() error = %v", err)
	}
	if users.updateCalls != 1 {
		t.Fatalf("generic updates = %d, want 1", users.updateCalls)
	}
}

func TestGenericUpdateEmailOnlyPreservesExistingPasswordValidation(t *testing.T) {
	users := &panicOnGenericUpdateFullUserRepository{}
	hasher := &statusStateMachinePasswordHasher{}
	usecase := NewUsecase(Dependencies{
		UserRepository: users,
		StatusReader:   &panicOnGenericUpdateStatusReader{},
		UpdateWriter:   users,
		PasswordHasher: hasher,
		Logger:         discardUserLogger{},
	})

	err := usecase.UpdateUser(context.Background(), UpdateInput{
		UserID: "user-1",
		Email:  "updated@example.com",
	})

	if !errors.Is(err, domainErrors.ErrPasswordTooShort) {
		t.Fatalf("UpdateUser(email only) error = %v, want ErrPasswordTooShort", err)
	}
	if hasher.calls != 0 || users.updateCalls != 0 {
		t.Fatalf("hash/update calls = %d/%d, want 0/0", hasher.calls, users.updateCalls)
	}
}

func TestGenericUpdatePasswordOnlyPreservesExistingBehavior(t *testing.T) {
	users := &panicOnGenericUpdateFullUserRepository{}
	usecase := NewUsecase(Dependencies{
		UserRepository: users,
		StatusReader:   &panicOnGenericUpdateStatusReader{},
		UpdateWriter:   users,
		PasswordHasher: &statusStateMachinePasswordHasher{},
		Logger:         discardUserLogger{},
	})

	err := usecase.UpdateUser(context.Background(), UpdateInput{
		UserID:   "user-1",
		Password: "Password1!",
	})

	if err != nil {
		t.Fatalf("UpdateUser(password only) error = %v", err)
	}
	if users.updateCalls != 1 || users.command.Email != "" || users.command.PasswordDigest.IsZero() {
		t.Fatalf("generic update command = %#v, want password-only mutation", users.command)
	}
}

type panicOnGenericUpdateFullUserRepository struct {
	repository.UserRepository
	updateCalls int
	command     UserUpdateCommand
}

func (*panicOnGenericUpdateFullUserRepository) FindByID(
	context.Context,
	string,
) (*entity.User, error) {
	panic("generic update must not load a full User aggregate")
}

func (r *panicOnGenericUpdateFullUserRepository) UpdateUser(
	_ context.Context,
	command UserUpdateCommand,
) error {
	r.updateCalls++
	r.command = command
	return nil
}

type panicOnGenericUpdateStatusReader struct{}

func (*panicOnGenericUpdateStatusReader) FindStatusSnapshotByID(
	context.Context,
	string,
) (UserStatusSnapshot, error) {
	panic("generic update must not load status")
}
