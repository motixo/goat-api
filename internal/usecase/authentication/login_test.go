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
)

func TestLoginUnknownEmailUsesInvalidCredentialsIdentity(t *testing.T) {
	users := &loginUserRepository{}
	usecase := NewUsecase(Dependencies{UserRepository: users, Logger: discardAuthLogger{}})

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

func TestLoginKeepsPersistedDigestRehydrationFailurePrivate(t *testing.T) {
	lookupErr := fmt.Errorf("map persisted user credential: %w", service.ErrInvalidStoredPasswordHash)
	users := &loginUserRepository{findByEmailErr: lookupErr}
	usecase := NewUsecase(Dependencies{UserRepository: users, Logger: discardAuthLogger{}})

	_, err := usecase.Login(context.Background(), LoginInput{
		Email:    "user@example.com",
		Password: "Password1!",
	})

	if !errors.Is(err, domainErrors.ErrInvalidCredentials) {
		t.Fatalf("Login() error = %v, want privacy-preserving ErrInvalidCredentials", err)
	}
	if !errors.Is(err, service.ErrInvalidStoredPasswordHash) {
		t.Fatalf("Login() error = %v, want retained invalid stored-hash identity", err)
	}
}

func TestLoginPreservesUnknownEmailLookupFailure(t *testing.T) {
	lookupErr := errors.New("postgres connection unavailable")
	users := &loginUserRepository{findByEmailErr: lookupErr}
	usecase := NewUsecase(Dependencies{UserRepository: users, Logger: discardAuthLogger{}})

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
