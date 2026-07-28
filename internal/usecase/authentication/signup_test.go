package authentication

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
	usecase := NewUsecase(Dependencies{
		UserRepository: &signupUserRepository{createErr: persistenceErr},
		PasswordHasher: signupPasswordHasher{},
		Logger:         discardAuthLogger{},
	})

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
	usecase := NewUsecase(Dependencies{
		UserRepository: repository,
		PasswordHasher: signupPasswordHasher{},
		Logger:         discardAuthLogger{},
	})

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
	if repository.created.PasswordDigest.IsZero() ||
		repository.created.PasswordDigest.Encoded() == "Password1!" {
		t.Fatal("Signup() persisted plaintext or omitted the password digest")
	}
	if output.Status != valueobject.StatusInactive.String() {
		t.Fatalf("output status = %q, want inactive", output.Status)
	}
}

func TestSignupDoesNotTrustArgonPrefixAsStoredDigest(t *testing.T) {
	repository := &signupUserRepository{}
	hasher := &recordingSignupPasswordHasher{}
	usecase := NewUsecase(Dependencies{
		UserRepository: repository,
		PasswordHasher: hasher,
		Logger:         discardAuthLogger{},
	})

	_, err := usecase.Signup(context.Background(), RegisterInput{
		Email:    "new@example.com",
		Password: "$argon2id$",
	})
	if !errors.Is(err, domainErrors.ErrPasswordPolicyViolation) {
		t.Fatalf("Signup() error = %v, want ErrPasswordPolicyViolation", err)
	}
	if hasher.calls != 0 {
		t.Fatalf("password hashes = %d, want 0 for invalid plaintext", hasher.calls)
	}
	if repository.created != nil {
		t.Fatal("Signup() persisted invalid hash-looking plaintext")
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

type recordingSignupPasswordHasher struct {
	service.PasswordHasher
	calls int
}

func (h *recordingSignupPasswordHasher) Hash(
	context.Context,
	valueobject.PlainPassword,
) (valueobject.PasswordDigest, error) {
	h.calls++
	return testPasswordDigest("$argon2id$recorded-signup-test"), nil
}

func (signupPasswordHasher) Hash(
	context.Context,
	valueobject.PlainPassword,
) (valueobject.PasswordDigest, error) {
	return testPasswordDigest("$argon2id$signup-test"), nil
}
