package auth

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/motixo/goat-api/internal/domain/entity"
	domainErrors "github.com/motixo/goat-api/internal/domain/errors"
	"github.com/motixo/goat-api/internal/domain/repository"
	"github.com/motixo/goat-api/internal/domain/service"
	"github.com/motixo/goat-api/internal/domain/valueobject"
)

func TestSignupPreservesSemanticEmailConflictAndPostgresCause(t *testing.T) {
	postgresCause := errors.New("postgres email unique violation")
	persistenceErr := fmt.Errorf("%w: %w", domainErrors.ErrEmailAlreadyExists, postgresCause)
	usecase := NewUsecase(
		&signupUserRepository{createErr: persistenceErr},
		nil,
		nil,
		signupPasswordHasher{},
		nil,
		discardAuthLogger{},
		0,
		0,
		0,
	)

	_, err := usecase.Signup(context.Background(), RegisterInput{
		Email:    "duplicate@example.com",
		Password: "Password1!",
	})

	if !errors.Is(err, persistenceErr) {
		t.Fatalf("Signup() error = %v, want repository error preserved", err)
	}
	if !errors.Is(err, domainErrors.ErrEmailAlreadyExists) {
		t.Fatalf("errors.Is(ErrEmailAlreadyExists) = false; error = %v", err)
	}
	if !errors.Is(err, postgresCause) {
		t.Fatalf("Signup() error did not preserve PostgreSQL cause: %v", err)
	}
}

func TestSignupCreatesInactiveUserWithoutSessionOrToken(t *testing.T) {
	repository := &signupUserRepository{}
	usecase := NewUsecase(
		repository,
		nil,
		nil,
		signupPasswordHasher{},
		nil,
		discardAuthLogger{},
		0,
		0,
		0,
	)

	output, err := usecase.Signup(context.Background(), RegisterInput{
		Email:    "new@example.com",
		Password: "Password1!",
	})
	if err != nil {
		t.Fatalf("Signup() error = %v", err)
	}
	if repository.created == nil {
		t.Fatal("Signup() did not persist a user")
	}
	if repository.created.Status != valueobject.StatusInactive {
		t.Fatalf(
			"persisted status = %s, want inactive",
			repository.created.Status,
		)
	}
	if output.Status != valueobject.StatusInactive.String() {
		t.Fatalf("output status = %q, want inactive", output.Status)
	}
}

type signupUserRepository struct {
	repository.UserRepository
	createErr error
	created   *entity.User
}

func (r *signupUserRepository) Create(_ context.Context, user *entity.User) error {
	r.created = user
	return r.createErr
}

type signupPasswordHasher struct {
	service.PasswordHasher
}

func (signupPasswordHasher) Hash(context.Context, string) (valueobject.Password, error) {
	return valueobject.PasswordFromHash("$argon2id$signup-test"), nil
}
