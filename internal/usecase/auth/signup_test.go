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
		signupPasswordHasher{},
		nil,
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

type signupUserRepository struct {
	repository.UserRepository
	createErr error
}

func (r *signupUserRepository) Create(context.Context, *entity.User) error {
	return r.createErr
}

type signupPasswordHasher struct {
	service.PasswordHasher
}

func (signupPasswordHasher) Hash(context.Context, string) (valueobject.Password, error) {
	return valueobject.PasswordFromHash("$argon2id$signup-test"), nil
}
