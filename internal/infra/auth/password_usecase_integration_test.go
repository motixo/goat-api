package service_test

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/motixo/goat-api/internal/domain/entity"
	domainErrors "github.com/motixo/goat-api/internal/domain/errors"
	"github.com/motixo/goat-api/internal/domain/repository"
	domainService "github.com/motixo/goat-api/internal/domain/service"
	"github.com/motixo/goat-api/internal/domain/valueobject"
	authInfra "github.com/motixo/goat-api/internal/infra/auth"
	"github.com/motixo/goat-api/internal/usecase/authentication"
	"github.com/motixo/goat-api/internal/usecase/authorization"
)

func TestLoginWithCorruptedPersistedHashPreservesPrivacyAndInternalIdentity(t *testing.T) {
	passwordHasher, err := authInfra.NewPasswordService(authInfra.PasswordHasherConfig{
		Pepper:         "corrupted-login-integration-pepper",
		MaxConcurrency: 1,
	})
	if err != nil {
		t.Fatalf("construct password hasher: %v", err)
	}
	encodedBytes := base64.RawStdEncoding.EncodeToString(make([]byte, 32))
	corruptedHash := fmt.Sprintf(
		"$argon2id$v=19$m=65537,t=3,p=4$%s$%s",
		encodedBytes,
		encodedBytes,
	)
	users := &passwordHasherIntegrationUserRepository{user: &entity.User{
		ID:                "corrupted-credential-user",
		Email:             "corrupted@example.com",
		PasswordDigest:    integrationPasswordDigest(corruptedHash),
		Status:            valueobject.StatusActive,
		Role:              valueobject.RoleClient,
		CredentialVersion: entity.InitialCredentialVersion,
	}}
	securityStates := &passwordHasherIntegrationSecurityStateReader{}
	usecase := authentication.NewUsecase(authentication.Dependencies{
		UserRepository:      users,
		SecurityStateReader: securityStates,
		PasswordHasher:      passwordHasher,
		Logger:              discardPasswordIntegrationLogger{},
	})

	_, err = usecase.Login(context.Background(), authentication.LoginInput{
		Email: users.user.Email, Password: "Password1!",
	})
	if !errors.Is(err, domainErrors.ErrInvalidCredentials) {
		t.Fatalf("Login() error = %v, want privacy-preserving ErrInvalidCredentials", err)
	}
	if !errors.Is(err, domainService.ErrInvalidStoredPasswordHash) {
		t.Fatalf("Login() error = %v, want invalid stored-hash identity", err)
	}
	if securityStates.calls != 0 {
		t.Fatalf("corrupted-hash security-state reads = %d, want 0", securityStates.calls)
	}
}

func TestSignupAndLoginUseBoundedPasswordHasherCompatibility(t *testing.T) {
	passwordHasher, err := authInfra.NewPasswordService(authInfra.PasswordHasherConfig{
		Pepper:         "signup-login-integration-pepper",
		MaxConcurrency: 1,
	})
	if err != nil {
		t.Fatalf("construct password hasher: %v", err)
	}
	users := &passwordHasherIntegrationUserRepository{}
	securityStateErr := errors.New("stop after successful password verification")
	securityStates := &passwordHasherIntegrationSecurityStateReader{err: securityStateErr}
	usecase := authentication.NewUsecase(authentication.Dependencies{
		UserRepository:      users,
		SecurityStateReader: securityStates,
		PasswordHasher:      passwordHasher,
		Logger:              discardPasswordIntegrationLogger{},
	})

	const plaintext = "Password1!"
	if _, err := usecase.Signup(context.Background(), authentication.RegisterInput{
		Email: "bounded-hasher@example.com", Password: plaintext,
	}); err != nil {
		t.Fatalf("Signup() error = %v", err)
	}
	if users.user == nil {
		t.Fatal("Signup() did not persist a user")
	}
	if users.user.PasswordDigest.Encoded() == plaintext ||
		!strings.HasPrefix(users.user.PasswordDigest.Encoded(), "$argon2id$") {
		t.Fatal("Signup() did not persist the compatible Argon2id encoding")
	}
	users.user.Status = valueobject.StatusActive

	_, err = usecase.Login(context.Background(), authentication.LoginInput{
		Email: users.user.Email, Password: "WrongPassword1!",
	})
	if !errors.Is(err, domainErrors.ErrInvalidCredentials) {
		t.Fatalf("Login() incorrect-password error = %v, want ErrInvalidCredentials", err)
	}
	if securityStates.calls != 0 {
		t.Fatalf("incorrect-password security-state reads = %d, want 0", securityStates.calls)
	}

	_, err = usecase.Login(context.Background(), authentication.LoginInput{
		Email: users.user.Email, Password: plaintext,
	})
	if !errors.Is(err, securityStateErr) {
		t.Fatalf("Login() error = %v, want post-verification sentinel", err)
	}
	if securityStates.calls != 1 {
		t.Fatalf("valid-password security-state reads = %d, want 1", securityStates.calls)
	}
}

type passwordHasherIntegrationUserRepository struct {
	repository.UserRepository
	user *entity.User
}

func (r *passwordHasherIntegrationUserRepository) Create(_ context.Context, user *entity.User) error {
	r.user = user
	return nil
}

func (r *passwordHasherIntegrationUserRepository) FindByEmail(
	context.Context,
	string,
) (*entity.User, error) {
	return r.user, nil
}

type passwordHasherIntegrationSecurityStateReader struct {
	calls int
	err   error
}

func (r *passwordHasherIntegrationSecurityStateReader) GetSecurityState(
	context.Context,
	string,
) (authorization.SecurityState, error) {
	r.calls++
	return authorization.SecurityState{}, r.err
}

type discardPasswordIntegrationLogger struct{}

func (discardPasswordIntegrationLogger) Info(string, ...any)  {}
func (discardPasswordIntegrationLogger) Error(string, ...any) {}
func (discardPasswordIntegrationLogger) Warn(string, ...any)  {}
func (discardPasswordIntegrationLogger) Debug(string, ...any) {}
func (discardPasswordIntegrationLogger) Panic(string, ...any) {}

func integrationPasswordDigest(encoded string) valueobject.PasswordDigest {
	digest, err := valueobject.NewPasswordDigest(encoded)
	if err != nil {
		panic("test password digest is invalid")
	}
	return digest
}
