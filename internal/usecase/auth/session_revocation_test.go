package auth

import (
	"context"
	stdErrors "errors"
	"reflect"
	"testing"
	"time"

	"github.com/motixo/goat-api/internal/domain/entity"
	"github.com/motixo/goat-api/internal/domain/repository"
	"github.com/motixo/goat-api/internal/domain/service"
	"github.com/motixo/goat-api/internal/domain/valueobject"
	"github.com/motixo/goat-api/internal/usecase/session"
)

func TestLogoutScopesCurrentSessionRevocationToAuthenticatedUser(t *testing.T) {
	sessions := &recordingAuthSessionUseCase{}
	usecase := NewUsecase(nil, sessions, nil, nil, nil, discardAuthLogger{}, 0, 0, 0)

	err := usecase.Logout(context.Background(), "01ARZ3NDEKTSV4RRFFQ69G5FAV", "user-1")

	if err != nil {
		t.Fatalf("Logout() error = %v", err)
	}
	want := session.DeleteSessionsInput{
		UserID:         "user-1",
		TargetSessions: []string{"01ARZ3NDEKTSV4RRFFQ69G5FAV"},
	}
	if !reflect.DeepEqual(sessions.deleteInput, want) {
		t.Fatalf("DeleteSessions input = %#v, want %#v", sessions.deleteInput, want)
	}
}

func TestLoginAccessTokenFailureScopesCleanupToCreatedSessionOwner(t *testing.T) {
	accessErr := stdErrors.New("access token generation failed")
	userEntity := &entity.User{
		ID:       "user-1",
		Email:    "user@example.com",
		Password: valueobject.PasswordFromHash("$hash"),
		Status:   valueobject.StatusActive,
	}
	users := &authUserRepository{user: userEntity}
	sessions := &recordingAuthSessionUseCase{}
	tokens := &authJWTService{accessErr: accessErr}
	passwords := &authPasswordHasher{}
	usecase := NewUsecase(
		users,
		sessions,
		passwords,
		tokens,
		nil,
		discardAuthLogger{},
		AccessTTL(time.Minute),
		RefreshTTL(time.Hour),
		SessionTTL(24*time.Hour),
	)

	_, err := usecase.Login(context.Background(), LoginInput{
		Email:    userEntity.Email,
		Password: "Password1!",
	})

	if !stdErrors.Is(err, accessErr) {
		t.Fatalf("Login() error = %v, want access-token error", err)
	}
	if sessions.createInput.ID == "" {
		t.Fatal("created session ID is empty")
	}
	wantDelete := session.DeleteSessionsInput{
		UserID:         userEntity.ID,
		TargetSessions: []string{sessions.createInput.ID},
	}
	if !reflect.DeepEqual(sessions.deleteInput, wantDelete) {
		t.Fatalf("cleanup input = %#v, want %#v", sessions.deleteInput, wantDelete)
	}
}

type authUserRepository struct {
	repository.UserRepository
	user *entity.User
}

func (r *authUserRepository) FindByEmail(context.Context, string) (*entity.User, error) {
	return r.user, nil
}

type recordingAuthSessionUseCase struct {
	session.UseCase
	createInput session.CreateInput
	deleteInput session.DeleteSessionsInput
}

func (s *recordingAuthSessionUseCase) CreateSession(_ context.Context, input session.CreateInput) error {
	s.createInput = input
	return nil
}

func (s *recordingAuthSessionUseCase) DeleteSessions(_ context.Context, input session.DeleteSessionsInput) error {
	s.deleteInput = input
	return nil
}

type authPasswordHasher struct {
	service.PasswordHasher
}

func (*authPasswordHasher) Verify(context.Context, string, valueobject.Password) bool {
	return true
}

type authJWTService struct {
	service.JWTService
	accessErr error
}

func (*authJWTService) GenerateRefreshToken(string, string, time.Duration) (string, *valueobject.JWTClaims, error) {
	return "refresh-token", &valueobject.JWTClaims{ExpiresAt: time.Now().Add(time.Hour)}, nil
}

func (s *authJWTService) GenerateAccessToken(string, string, string, time.Duration) (string, *valueobject.JWTClaims, error) {
	return "", nil, s.accessErr
}

type discardAuthLogger struct{}

func (discardAuthLogger) Info(string, ...any)  {}
func (discardAuthLogger) Error(string, ...any) {}
func (discardAuthLogger) Warn(string, ...any)  {}
func (discardAuthLogger) Debug(string, ...any) {}
func (discardAuthLogger) Panic(string, ...any) {}
