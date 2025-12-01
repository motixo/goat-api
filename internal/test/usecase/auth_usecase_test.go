package auth_test

import (
	"context"
	"testing"
	"time"

	"github.com/mot0x0/goth-api/internal/config"
	"github.com/mot0x0/goth-api/internal/domain/entity"
	"github.com/mot0x0/goth-api/internal/domain/service"
	authUC "github.com/mot0x0/goth-api/internal/domain/usecase/auth"
	"github.com/mot0x0/goth-api/internal/domain/usecase/session"
	"github.com/mot0x0/goth-api/internal/domain/usecase/user"
	"github.com/mot0x0/goth-api/internal/domain/valueobject"
)

// --- Mock ---

type MockUserRepo struct {
	CreateFn      func(ctx context.Context, u *entity.User) error
	FindByEmailFn func(ctx context.Context, email string) (*entity.User, error)
	FindByIDFn    func(ctx context.Context, id string) (*entity.User, error)
}

func (m *MockUserRepo) Create(ctx context.Context, u *entity.User) error {
	return m.CreateFn(ctx, u)
}

func (m *MockUserRepo) FindByEmail(ctx context.Context, email string) (*entity.User, error) {
	return m.FindByEmailFn(ctx, email)
}

func (m *MockUserRepo) FindByID(ctx context.Context, id string) (*entity.User, error) {
	return m.FindByIDFn(ctx, id)
}

type MockPasswordService struct {
	HashFn   func(ctx context.Context, plain string) (valueobject.Password, error)
	VerifyFn func(ctx context.Context, plain string, hashed valueobject.Password) bool
}

func (m *MockPasswordService) Hash(ctx context.Context, plain string) (valueobject.Password, error) {
	return m.HashFn(ctx, plain)
}

func (m *MockPasswordService) Verify(ctx context.Context, plain string, hashed valueobject.Password) bool {
	return m.VerifyFn(ctx, plain, hashed)
}

type MockSessionUC struct {
	CreateSessionFn     func(ctx context.Context, input session.CreateInput) (string, error)
	DeleteSessionsFn    func(ctx context.Context, input session.DeleteSessionsInput) error
	RotateFn            func(ctx context.Context, input session.RotateInput) (string, error)
	GetSessionsByUserFn func(ctx context.Context, userID, sessionID string) ([]*session.SessionResponse, error)
	IsJTIValidFn        func(ctx context.Context, jti string) (bool, error)
}

func (m *MockSessionUC) CreateSession(ctx context.Context, input session.CreateInput) (string, error) {
	return m.CreateSessionFn(ctx, input)
}

func (m *MockSessionUC) DeleteSessions(ctx context.Context, input session.DeleteSessionsInput) error {
	return m.DeleteSessionsFn(ctx, input)
}

func (m *MockSessionUC) RotateSessionJTI(ctx context.Context, input session.RotateInput) (string, error) {
	return m.RotateFn(ctx, input)
}

func (m *MockSessionUC) GetSessionsByUser(ctx context.Context, userID, sessionID string) ([]*session.SessionResponse, error) {
	return m.GetSessionsByUserFn(ctx, userID, sessionID)
}

func (m *MockSessionUC) IsJTIValid(ctx context.Context, jti string) (bool, error) {
	return m.IsJTIValidFn(ctx, jti)
}

type MockLogger struct{}

func (m *MockLogger) Info(msg string, fields ...any)  {}
func (m *MockLogger) Error(msg string, fields ...any) {}
func (m *MockLogger) Warn(msg string, fields ...any)  {}
func (m *MockLogger) Debug(msg string, fields ...any) {}
func (m *MockLogger) Panic(msg string, fields ...any) {}

// --- Helpers ---

func newTestAuthUseCase(
	userRepo user.Repository,
	passwordHasher service.PasswordHasher,
	sessionUC session.UseCase,
	ulidGen *service.ULIDGenerator,
	secret string,
) authUC.UseCase {
	cfg := &config.Config{
		Env:            "development",
		ServerPort:     "8080",
		DBHost:         "localhost",
		DBPort:         "5432",
		DBUser:         "test",
		DBPassword:     "test",
		DBName:         "testdb",
		JWTSecret:      "secret",
		PasswordPepper: "pepper",
		RedisAddr:      "localhost:6379",
		RedisPassword:  "",
		RedisDB:        0,
		JWTExpiration:  time.Hour,
		GinMode:        "debug",
	}
	return authUC.NewUsecase(userRepo, sessionUC, passwordHasher, &MockLogger{}, ulidGen, cfg)
}

// --- Tests ---

func TestSignup_Success(t *testing.T) {
	ctx := context.Background()
	userRepo := &MockUserRepo{
		CreateFn:      func(ctx context.Context, u *entity.User) error { return nil },
		FindByEmailFn: func(ctx context.Context, email string) (*entity.User, error) { return nil, nil },
		FindByIDFn:    func(ctx context.Context, id string) (*entity.User, error) { return nil, nil },
	}

	passwordSvc := &MockPasswordService{
		HashFn: func(ctx context.Context, plain string) (valueobject.Password, error) {
			return valueobject.PasswordFromHash("hashed"), nil
		},
	}

	uc := newTestAuthUseCase(userRepo, passwordSvc, &MockSessionUC{}, service.NewULIDGenerator(), "secret")

	output, err := uc.Signup(ctx, authUC.RegisterInput{
		Email:    "test@example.com",
		Password: "Abc123!@#",
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if output.User.ID == "" {
		t.Fatal("expected non-empty user ID")
	}
}

func TestLogin_Success(t *testing.T) {
	ctx := context.Background()
	user := &entity.User{
		ID:        "uid123",
		Email:     "test@example.com",
		Password:  "hashed",
		CreatedAt: time.Now(),
	}

	userRepo := &MockUserRepo{
		FindByEmailFn: func(ctx context.Context, email string) (*entity.User, error) { return user, nil },
	}
	passwordSvc := &MockPasswordService{
		VerifyFn: func(ctx context.Context, plain string, hashed valueobject.Password) bool { return true },
	}
	sessionUC := &MockSessionUC{
		CreateSessionFn: func(ctx context.Context, input session.CreateInput) (string, error) { return "sess123", nil },
	}
	uc := newTestAuthUseCase(userRepo, passwordSvc, sessionUC, service.NewULIDGenerator(), "secret")

	output, err := uc.Login(ctx, authUC.LoginInput{
		Email:    "test@example.com",
		Password: "Abc123!@#",
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if output.AccessToken == "" || output.RefreshToken == "" {
		t.Fatal("expected access and refresh tokens")
	}
}

func TestRefresh_Success(t *testing.T) {
	ctx := context.Background()
	ulidGen := service.NewULIDGenerator()
	sessionUC := &MockSessionUC{
		RotateFn: func(ctx context.Context, input session.RotateInput) (string, error) { return "sess123", nil },
	}
	uc := newTestAuthUseCase(&MockUserRepo{}, &MockPasswordService{}, sessionUC, ulidGen, "secret")

	refreshToken, _, _ := valueobject.NewRefreshToken("uid123", "secret", ulidGen.New())

	output, err := uc.Refresh(ctx, authUC.RefreshInput{RefreshToken: refreshToken})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if output.AccessToken == "" || output.RefreshToken == "" {
		t.Fatal("expected access and refresh tokens")
	}
}

func TestLogout_Success(t *testing.T) {
	ctx := context.Background()
	sessionUC := &MockSessionUC{
		DeleteSessionsFn: func(ctx context.Context, input session.DeleteSessionsInput) error { return nil },
	}
	uc := newTestAuthUseCase(&MockUserRepo{}, &MockPasswordService{}, sessionUC, service.NewULIDGenerator(), "secret")

	err := uc.Logout(ctx, "sess123", "uid123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
