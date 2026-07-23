package user

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
	"github.com/motixo/goat-api/internal/config"
	"github.com/motixo/goat-api/internal/domain/entity"
	"github.com/motixo/goat-api/internal/domain/repository"
	"github.com/motixo/goat-api/internal/domain/service"
	"github.com/motixo/goat-api/internal/domain/valueobject"
	authInfra "github.com/motixo/goat-api/internal/infra/auth"
	postgresUser "github.com/motixo/goat-api/internal/infra/database/postgres/user"
	redisSession "github.com/motixo/goat-api/internal/infra/storage/redis/session"
	"github.com/motixo/goat-api/internal/pkg"
	authUseCase "github.com/motixo/goat-api/internal/usecase/auth"
	sessionUseCase "github.com/motixo/goat-api/internal/usecase/session"
	"github.com/redis/go-redis/v9"
)

func TestCredentialVersionIntegrationRejectsRetainedSessionAfterCleanupFailure(t *testing.T) {
	ctx, userRepository, passwordHasher, redisClient := newCredentialVersionIntegration(t)
	logRecorder := &passwordChangeLogRecorder{}
	logger := passwordChangeLogger{recorder: logRecorder}
	cleanupMetrics := &passwordChangeCleanupMetrics{}
	userID := createCredentialVersionIntegrationUser(t, ctx, userRepository, passwordHasher)

	oldSession := newPasswordChangeIntegrationSession(userID)
	sessionRepository := redisSession.NewRepository(redisClient, logger)
	if err := sessionRepository.Create(ctx, oldSession); err != nil {
		t.Fatalf("create old session: %v", err)
	}
	registerCredentialVersionSessionCleanup(t, redisClient, oldSession)

	if err := redisClient.Close(); err != nil {
		t.Fatalf("close cleanup Redis client: %v", err)
	}

	changePassword := NewUsecase(
		userRepository,
		passwordHasher,
		logger,
		sessionRepository,
		nil,
		nil,
		cleanupMetrics,
	)
	err := changePassword.ChangePassword(ctx, UpdatePassInput{
		UserID:      userID,
		OldPassword: passwordChangeOldPassword,
		NewPassword: passwordChangeNewPassword,
	})
	if err != nil {
		t.Fatalf("ChangePassword() error = %v after PostgreSQL commit and Redis cleanup failure", err)
	}
	assertPasswordCleanupFailureObserved(
		t,
		&passwordChangeFixture{logger: logRecorder, metrics: cleanupMetrics},
		passwordChangeCleanupStageSessionRevocation,
		nil,
	)

	persisted, err := userRepository.FindByID(ctx, userID)
	if err != nil {
		t.Fatalf("FindByID() error = %v", err)
	}
	if persisted.CredentialVersion != entity.InitialCredentialVersion+1 {
		t.Fatalf("credential version = %d, want 2", persisted.CredentialVersion)
	}
	if !passwordHasher.Verify(ctx, passwordChangeNewPassword, persisted.Password) {
		t.Fatal("new password was not committed before Redis cleanup failure")
	}

	validationClient := newCredentialVersionRedisClient(t)
	registerCredentialVersionSessionCleanup(t, validationClient, oldSession)
	if exists, err := validationClient.Exists(
		ctx,
		pkg.RedisKey("session", "id", oldSession.ID),
		pkg.RedisKey("session", "jti", oldSession.CurrentJTI),
	).Result(); err != nil {
		t.Fatalf("check retained Redis session: %v", err)
	} else if exists != 2 {
		t.Fatalf("retained Redis keys = %d, want 2 after cleanup failure", exists)
	}

	validation := sessionUseCase.NewUsecase(
		redisSession.NewRepository(validationClient, logger),
		userRepository,
		logger,
	)
	valid, err := validation.ValidateSession(ctx, sessionUseCase.ValidateInput{
		UserID:    oldSession.UserID,
		SessionID: oldSession.ID,
		JTI:       oldSession.CurrentJTI,
	})
	if err != nil {
		t.Fatalf("ValidateSession() error = %v", err)
	}
	if valid {
		t.Fatal("retained old-version Redis session remained valid")
	}
}

func TestCredentialVersionIntegrationClosesConcurrentLoginPasswordChangeRace(t *testing.T) {
	ctx, userRepository, passwordHasher, redisClient := newCredentialVersionIntegration(t)
	logger := passwordChangeLogger{}
	userID := createCredentialVersionIntegrationUser(t, ctx, userRepository, passwordHasher)
	persistedBefore, err := userRepository.FindByID(ctx, userID)
	if err != nil {
		t.Fatalf("FindByID(before race) error = %v", err)
	}

	sessionRepository := redisSession.NewRepository(redisClient, logger)
	sessions := sessionUseCase.NewUsecase(sessionRepository, userRepository, logger)
	snapshotRead := make(chan struct{})
	continueLogin := make(chan struct{})
	loginRepository := &blockingLoginUserRepository{
		UserRepository: userRepository,
		snapshotRead:   snapshotRead,
		continueLogin:  continueLogin,
	}
	jwtManager := authInfra.NewJWTManager("credential-version-integration-secret")
	login := authUseCase.NewUsecase(
		loginRepository,
		sessions,
		passwordHasher,
		jwtManager,
		nil,
		logger,
		authUseCase.AccessTTL(time.Minute),
		authUseCase.RefreshTTL(time.Hour),
		authUseCase.SessionTTL(time.Hour),
	)

	type loginResult struct {
		output authUseCase.LoginOutput
		err    error
	}
	loginDone := make(chan loginResult, 1)
	go func() {
		output, loginErr := login.Login(ctx, authUseCase.LoginInput{
			Email:    persistedBefore.Email,
			Password: passwordChangeOldPassword,
			IP:       "127.0.0.1",
			Device:   "concurrent-login",
		})
		loginDone <- loginResult{output: output, err: loginErr}
	}()

	select {
	case <-snapshotRead:
	case <-ctx.Done():
		t.Fatalf("wait for login credential snapshot: %v", ctx.Err())
	}

	changePassword := NewUsecase(
		userRepository,
		passwordHasher,
		logger,
		sessionRepository,
		nil,
		nil,
		nil,
	)
	if err := changePassword.ChangePassword(ctx, UpdatePassInput{
		UserID:      userID,
		OldPassword: passwordChangeOldPassword,
		NewPassword: passwordChangeNewPassword,
	}); err != nil {
		t.Fatalf("ChangePassword() error = %v", err)
	}
	close(continueLogin)

	var result loginResult
	select {
	case result = <-loginDone:
	case <-ctx.Done():
		t.Fatalf("wait for delayed login: %v", ctx.Err())
	}
	if result.err != nil {
		t.Fatalf("delayed Login() error = %v", result.err)
	}

	claims, err := jwtManager.ParseAndValidate(result.output.AccessToken)
	if err != nil {
		t.Fatalf("parse delayed login access token: %v", err)
	}
	stored, err := sessionRepository.FindByJTI(ctx, claims.JTI)
	if err != nil {
		t.Fatalf("FindByJTI(delayed login) error = %v", err)
	}
	if stored == nil {
		t.Fatal("delayed login did not create its Redis session")
	}
	registerCredentialVersionSessionCleanup(t, redisClient, stored)
	if stored.CredentialVersion != persistedBefore.CredentialVersion {
		t.Fatalf(
			"delayed session credential version = %d, want stale snapshot %d",
			stored.CredentialVersion,
			persistedBefore.CredentialVersion,
		)
	}

	authoritativeVersion, err := userRepository.GetCredentialVersion(ctx, userID)
	if err != nil {
		t.Fatalf("GetCredentialVersion() error = %v", err)
	}
	if authoritativeVersion != persistedBefore.CredentialVersion+1 {
		t.Fatalf(
			"authoritative version = %d, want %d",
			authoritativeVersion,
			persistedBefore.CredentialVersion+1,
		)
	}

	valid, err := sessions.ValidateSession(ctx, sessionUseCase.ValidateInput{
		UserID:    claims.UserID,
		SessionID: claims.SessionID,
		JTI:       claims.JTI,
	})
	if err != nil {
		t.Fatalf("ValidateSession(delayed login) error = %v", err)
	}
	if valid {
		t.Fatal("session created from pre-change credentials was usable after password commit")
	}
}

func newCredentialVersionIntegration(
	t *testing.T,
) (context.Context, repository.UserRepository, service.PasswordHasher, *redis.Client) {
	t.Helper()
	dsn := os.Getenv("GOAT_POSTGRES_TEST_DSN")
	if dsn == "" || os.Getenv("GOAT_REDIS_ADDR") == "" {
		t.Skip("set GOAT_POSTGRES_TEST_DSN and GOAT_REDIS_ADDR to run credential-version integration tests")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	t.Cleanup(cancel)
	db, err := sqlx.Connect("postgres", dsn)
	if err != nil {
		t.Fatalf("connect to PostgreSQL: %v", err)
	}
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	t.Cleanup(func() {
		if err := db.Close(); err != nil {
			t.Errorf("close PostgreSQL: %v", err)
		}
	})
	const schema = `
		CREATE TEMP TABLE users (
			id UUID PRIMARY KEY,
			email TEXT NOT NULL UNIQUE,
			password TEXT NOT NULL,
			status SMALLINT NOT NULL,
			role SMALLINT NOT NULL,
			credential_version BIGINT NOT NULL DEFAULT 1 CHECK (credential_version > 0),
			created_at TIMESTAMP NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMP NULL
		) ON COMMIT PRESERVE ROWS
	`
	if _, err := db.ExecContext(ctx, schema); err != nil {
		t.Fatalf("create temporary users table: %v", err)
	}

	passwordHasher := authInfra.NewPasswordService(&config.Config{PasswordPepper: "credential-version-pepper"})
	return ctx, postgresUser.NewRepository(db), passwordHasher, newCredentialVersionRedisClient(t)
}

func newCredentialVersionRedisClient(t *testing.T) *redis.Client {
	t.Helper()
	client := redis.NewClient(&redis.Options{Addr: os.Getenv("GOAT_REDIS_ADDR")})
	if err := client.Ping(context.Background()).Err(); err != nil {
		_ = client.Close()
		t.Fatalf("ping Redis: %v", err)
	}
	t.Cleanup(func() {
		if err := client.Close(); err != nil && !errors.Is(err, redis.ErrClosed) {
			t.Errorf("close Redis: %v", err)
		}
	})
	return client
}

func createCredentialVersionIntegrationUser(
	t *testing.T,
	ctx context.Context,
	users repository.UserRepository,
	passwordHasher service.PasswordHasher,
) string {
	t.Helper()
	password, err := passwordHasher.Hash(ctx, passwordChangeOldPassword)
	if err != nil {
		t.Fatalf("hash old password: %v", err)
	}
	userID := uuid.NewString()
	if err := users.Create(ctx, &entity.User{
		ID:                userID,
		Email:             "credential-version-" + userID + "@example.com",
		Password:          password,
		Status:            valueobject.StatusActive,
		Role:              valueobject.RoleClient,
		CredentialVersion: entity.InitialCredentialVersion,
		CreatedAt:         time.Now().UTC(),
	}); err != nil {
		t.Fatalf("create user: %v", err)
	}
	return userID
}

func registerCredentialVersionSessionCleanup(
	t *testing.T,
	client *redis.Client,
	current *entity.Session,
) {
	t.Helper()
	t.Cleanup(func() {
		_ = client.Del(
			context.Background(),
			pkg.RedisKey("session", "id", current.ID),
			pkg.RedisKey("session", "jti", current.CurrentJTI),
			pkg.RedisKey("session", "user", current.UserID),
		).Err()
	})
}

type blockingLoginUserRepository struct {
	repository.UserRepository
	snapshotRead  chan struct{}
	continueLogin chan struct{}
}

func (r *blockingLoginUserRepository) FindByEmail(
	ctx context.Context,
	email string,
) (*entity.User, error) {
	user, err := r.UserRepository.FindByEmail(ctx, email)
	close(r.snapshotRead)
	select {
	case <-r.continueLogin:
		return user, err
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}
