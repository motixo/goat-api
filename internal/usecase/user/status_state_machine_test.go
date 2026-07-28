package user

import (
	"context"
	"errors"
	"testing"

	"github.com/motixo/goat-api/internal/domain/entity"
	domainErrors "github.com/motixo/goat-api/internal/domain/errors"
	"github.com/motixo/goat-api/internal/domain/repository"
	"github.com/motixo/goat-api/internal/domain/service"
	"github.com/motixo/goat-api/internal/domain/valueobject"
)

func TestCreateUserRequiresInactiveInitialStatus(t *testing.T) {
	for _, status := range []valueobject.UserStatus{
		valueobject.StatusActive,
		valueobject.StatusSuspended,
		valueobject.StatusUnknown,
	} {
		t.Run(status.String(), func(t *testing.T) {
			users := &statusStateMachineUserRepository{}
			hasher := &statusStateMachinePasswordHasher{}
			usecase := NewUsecase(Dependencies{
				UserRepository: users,
				PasswordHasher: hasher,
				Logger:         discardUserLogger{},
			})

			_, err := usecase.CreateUser(context.Background(), CreateInput{
				Email:    "new@example.com",
				Password: "Password1!",
				Status:   status,
				Role:     valueobject.RoleClient,
			})

			if err == nil {
				t.Fatal("CreateUser() error = nil, want invalid initial status")
			}
			if !errors.Is(err, domainErrors.ErrInvalidUserStatusTransition) {
				t.Fatalf(
					"CreateUser() error = %v, want ErrInvalidUserStatusTransition",
					err,
				)
			}
			if users.createCalls != 0 {
				t.Fatalf("repository creates = %d, want 0", users.createCalls)
			}
			if hasher.calls != 0 {
				t.Fatalf("password hashes = %d, want 0", hasher.calls)
			}
		})
	}
}

func TestCreateUserPersistsInactiveInitialStatus(t *testing.T) {
	users := &statusStateMachineUserRepository{}
	usecase := NewUsecase(Dependencies{
		UserRepository: users,
		PasswordHasher: &statusStateMachinePasswordHasher{},
		Logger:         discardUserLogger{},
	})

	output, err := usecase.CreateUser(context.Background(), CreateInput{
		Email:    "new@example.com",
		Password: "Password1!",
		Status:   valueobject.StatusInactive,
		Role:     valueobject.RoleClient,
	})

	if err != nil {
		t.Fatalf("CreateUser() error = %v", err)
	}
	if users.created == nil || users.created.Status != valueobject.StatusInactive {
		t.Fatalf("persisted user = %#v, want inactive status", users.created)
	}
	if output.Status != valueobject.StatusInactive.String() {
		t.Fatalf("output status = %q, want inactive", output.Status)
	}
}

type statusStateMachineUserRepository struct {
	repository.UserRepository
	created     *entity.User
	createCalls int
}

func (r *statusStateMachineUserRepository) Create(
	_ context.Context,
	user *entity.User,
) error {
	r.createCalls++
	r.created = user
	return nil
}

type statusStateMachinePasswordHasher struct {
	service.PasswordHasher
	calls int
}

func (h *statusStateMachinePasswordHasher) Hash(
	context.Context,
	valueobject.PlainPassword,
) (valueobject.PasswordDigest, error) {
	h.calls++
	return testPasswordDigest("$argon2id$status-state-machine"), nil
}
