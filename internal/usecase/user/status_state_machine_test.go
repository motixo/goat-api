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
			usecase := NewUsecase(
				users,
				hasher,
				discardUserLogger{},
				nil,
				nil,
			)

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
	usecase := NewUsecase(
		users,
		&statusStateMachinePasswordHasher{},
		discardUserLogger{},
		nil,
		nil,
	)

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

func TestGenericUserUpdateCannotChangeStatus(t *testing.T) {
	users := &statusStateMachineUserRepository{
		found: &entity.User{
			ID:     "user-1",
			Status: valueobject.StatusActive,
		},
	}
	hasher := &statusStateMachinePasswordHasher{}
	usecase := NewUsecase(
		users,
		hasher,
		discardUserLogger{},
		nil,
		nil,
	)

	err := usecase.UpdateUser(context.Background(), UpdateInput{
		UserID:   "user-1",
		Email:    "updated@example.com",
		Password: "Password1!",
		Status:   valueobject.StatusInactive,
		Role:     valueobject.RoleOperator,
	})

	if err == nil {
		t.Fatal("UpdateUser() error = nil, want status transition rejection")
	}
	if !errors.Is(err, domainErrors.ErrInvalidUserStatusTransition) {
		t.Fatalf(
			"UpdateUser() error = %v, want ErrInvalidUserStatusTransition",
			err,
		)
	}
	if users.findCalls != 1 {
		t.Fatalf("repository finds = %d, want 1", users.findCalls)
	}
	if users.updateCalls != 0 {
		t.Fatalf("repository updates = %d, want 0", users.updateCalls)
	}
	if hasher.calls != 0 {
		t.Fatalf("password hashes = %d, want 0", hasher.calls)
	}
}

func TestGenericUserUpdateMayVerifyButDoesNotPersistCurrentStatus(t *testing.T) {
	users := &statusStateMachineUserRepository{
		found: &entity.User{
			ID:     "user-1",
			Status: valueobject.StatusActive,
		},
	}
	usecase := NewUsecase(
		users,
		&statusStateMachinePasswordHasher{},
		discardUserLogger{},
		nil,
		nil,
	)

	err := usecase.UpdateUser(context.Background(), UpdateInput{
		UserID:   "user-1",
		Email:    "updated@example.com",
		Password: "Password1!",
		Status:   valueobject.StatusActive,
		Role:     valueobject.RoleOperator,
	})

	if err != nil {
		t.Fatalf("UpdateUser() error = %v", err)
	}
	if users.findCalls != 1 || users.updateCalls != 1 {
		t.Fatalf(
			"repository calls: find=%d update=%d, want 1/1",
			users.findCalls,
			users.updateCalls,
		)
	}
	if users.updated == nil || users.updated.Status != valueobject.StatusUnknown {
		t.Fatalf(
			"updated user = %#v, want status excluded from generic persistence",
			users.updated,
		)
	}
}

type statusStateMachineUserRepository struct {
	repository.UserRepository
	found       *entity.User
	findErr     error
	created     *entity.User
	updated     *entity.User
	createCalls int
	findCalls   int
	updateCalls int
}

func (r *statusStateMachineUserRepository) Create(
	_ context.Context,
	user *entity.User,
) error {
	r.createCalls++
	r.created = user
	return nil
}

func (r *statusStateMachineUserRepository) FindByID(
	context.Context,
	string,
) (*entity.User, error) {
	r.findCalls++
	return r.found, r.findErr
}

func (r *statusStateMachineUserRepository) Update(
	_ context.Context,
	user *entity.User,
) error {
	r.updateCalls++
	r.updated = user
	return nil
}

type statusStateMachinePasswordHasher struct {
	service.PasswordHasher
	calls int
}

func (h *statusStateMachinePasswordHasher) Hash(
	context.Context,
	string,
) (valueobject.Password, error) {
	h.calls++
	return valueobject.PasswordFromHash("$argon2id$status-state-machine"), nil
}
