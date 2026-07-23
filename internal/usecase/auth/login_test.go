package auth

import (
	"context"
	"errors"
	"testing"

	"github.com/motixo/goat-api/internal/domain/entity"
	domainErrors "github.com/motixo/goat-api/internal/domain/errors"
	"github.com/motixo/goat-api/internal/domain/repository"
)

func TestLoginUnknownEmailUsesInvalidCredentialsIdentity(t *testing.T) {
	users := &loginUserRepository{}
	usecase := NewUsecase(users, nil, nil, nil, nil, discardAuthLogger{}, 0, 0, 0)

	_, err := usecase.Login(context.Background(), LoginInput{
		Email:    "missing@example.com",
		Password: "Password1!",
	})

	if !errors.Is(err, domainErrors.ErrInvalidCredentials) {
		t.Fatalf("Login() error = %v, want ErrInvalidCredentials", err)
	}
	if errors.Is(err, domainErrors.ErrNotFound) || errors.Is(err, domainErrors.ErrUserNotFound) {
		t.Fatalf("Login() exposed missing-email identity: %v", err)
	}
	if users.findByEmailCalls != 1 {
		t.Fatalf("FindByEmail calls = %d, want 1", users.findByEmailCalls)
	}
}

func TestLoginPreservesUnknownEmailLookupFailure(t *testing.T) {
	lookupErr := errors.New("postgres connection unavailable")
	users := &loginUserRepository{findByEmailErr: lookupErr}
	usecase := NewUsecase(users, nil, nil, nil, nil, discardAuthLogger{}, 0, 0, 0)

	_, err := usecase.Login(context.Background(), LoginInput{
		Email:    "user@example.com",
		Password: "Password1!",
	})

	if !errors.Is(err, lookupErr) {
		t.Fatalf("Login() error = %v, want lookup failure", err)
	}
	if errors.Is(err, domainErrors.ErrInvalidCredentials) {
		t.Fatalf("Login() misclassified PostgreSQL failure as invalid credentials: %v", err)
	}
}

type loginUserRepository struct {
	repository.UserRepository
	user             *entity.User
	findByEmailErr   error
	findByEmailCalls int
}

func (r *loginUserRepository) FindByEmail(context.Context, string) (*entity.User, error) {
	r.findByEmailCalls++
	return r.user, r.findByEmailErr
}
