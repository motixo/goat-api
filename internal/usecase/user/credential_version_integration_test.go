package user

import (
	"context"
	"errors"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/jmoiron/sqlx"
	"github.com/motixo/goat-api/internal/config"
	"github.com/motixo/goat-api/internal/domain/entity"
	domainErrors "github.com/motixo/goat-api/internal/domain/errors"
	"github.com/motixo/goat-api/internal/domain/repository"
	"github.com/motixo/goat-api/internal/domain/service"
	"github.com/motixo/goat-api/internal/domain/valueobject"
	authInfra "github.com/motixo/goat-api/internal/infra/auth"
	postgresUser "github.com/motixo/goat-api/internal/infra/database/postgres/user"
	redisSession "github.com/motixo/goat-api/internal/infra/storage/redis/session"
	"github.com/motixo/goat-api/internal/pkg"
	authUseCase "github.com/motixo/goat-api/internal/usecase/auth"
	"github.com/motixo/goat-api/internal/usecase/authorization"
	sessionUseCase "github.com/motixo/goat-api/internal/usecase/session"
	"github.com/redis/go-redis/v9"
)

func TestCredentialVersionIntegrationRetainedSessionPassesSnapshotButFailsFreshAuthorization(t *testing.T) {
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
		logger,
	)
	valid, err := validation.ValidateSession(ctx, sessionUseCase.ValidateInput{
		UserID:            oldSession.UserID,
		SessionID:         oldSession.ID,
		JTI:               oldSession.CurrentJTI,
		CredentialVersion: oldSession.CredentialVersion,
	})
	if err != nil {
		t.Fatalf("ValidateSession() error = %v", err)
	}
	if !valid {
		t.Fatal("ordinary Redis snapshot validation unexpectedly rejected the retained session")
	}

	permissions, err := valueobject.NewPermissionSet(nil)
	if err != nil {
		t.Fatalf("build empty permission set: %v", err)
	}
	principal, err := authorization.NewPrincipal(
		oldSession.UserID,
		oldSession.ID,
		oldSession.CredentialVersion,
		valueobject.RoleClient,
		permissions,
	)
	if err != nil {
		t.Fatalf("build old principal: %v", err)
	}
	_, err = authorization.NewUsecase(nil, userRepository).AuthorizeFresh(
		ctx,
		principal,
		"",
	)
	if !errors.Is(err, authorization.ErrPrincipalSecurityStateChanged) {
		t.Fatalf("fresh authorization error = %v, want credential-version rejection", err)
	}
}

func TestCredentialVersionIntegrationBoundsConcurrentLoginPasswordChangeRace(t *testing.T) {
	ctx, userRepository, passwordHasher, redisClient := newCredentialVersionIntegration(t)
	logger := passwordChangeLogger{}
	userID := createCredentialVersionIntegrationUser(t, ctx, userRepository, passwordHasher)
	persistedBefore, err := userRepository.FindByID(ctx, userID)
	if err != nil {
		t.Fatalf("FindByID(before race) error = %v", err)
	}

	sessionRepository := redisSession.NewRepository(redisClient, logger)
	sessions := sessionUseCase.NewUsecase(sessionRepository, logger)
	snapshotRead := make(chan struct{})
	continueLogin := make(chan struct{})
	securityStates := &blockingSecurityStateReader{
		SecurityStateReader: userRepository,
		snapshotRead:        snapshotRead,
		continueLogin:       continueLogin,
	}
	jwtManager := authInfra.NewJWTManager("credential-version-integration-secret")
	login := authUseCase.NewUsecase(
		userRepository,
		securityStates,
		sessions,
		passwordHasher,
		jwtManager,
		logger,
		authUseCase.AccessTTL(5*time.Minute),
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
		t.Fatalf("wait for login authorization snapshot: %v", ctx.Err())
	}

	changePassword := NewUsecase(
		userRepository,
		passwordHasher,
		logger,
		sessionRepository,
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
	stored, err := sessionRepository.FindByJTI(
		ctx,
		claims.JTI,
		claims.UserID,
		claims.SessionID,
		claims.CredentialVersion,
	)
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
		UserID:            claims.UserID,
		SessionID:         claims.SessionID,
		JTI:               claims.JTI,
		CredentialVersion: claims.CredentialVersion,
	})
	if err != nil {
		t.Fatalf("ValidateSession(delayed login) error = %v", err)
	}
	if !valid {
		t.Fatal("ordinary snapshot validation rejected the bounded stale login session")
	}

	principal, err := authorization.NewPrincipal(
		claims.UserID,
		claims.SessionID,
		claims.CredentialVersion,
		claims.Role,
		claims.Permissions,
	)
	if err != nil {
		t.Fatalf("build delayed-login principal: %v", err)
	}
	if _, err := authorization.NewUsecase(nil, userRepository).AuthorizeFresh(
		ctx,
		principal,
		"",
	); !errors.Is(err, authorization.ErrPrincipalSecurityStateChanged) {
		t.Fatalf("fresh authorization error = %v, want credential-version rejection", err)
	}

	if _, err := login.Refresh(ctx, authUseCase.RefreshInput{
		RefreshToken: result.output.RefreshToken,
		IP:           "127.0.0.1",
		Device:       "concurrent-refresh",
	}); !errors.Is(err, domainErrors.ErrUnauthorized) {
		t.Fatalf("Refresh(stale login) error = %v, want unauthorized", err)
	}
}

func TestSuspensionIntegrationBlocksLoginAfterAuthoritativeStateWasLoaded(t *testing.T) {
	ctx, userRepository, passwordHasher, redisClient := newCredentialVersionIntegration(t)
	logger := passwordChangeLogger{}
	userID := createCredentialVersionIntegrationUser(t, ctx, userRepository, passwordHasher)
	t.Cleanup(func() {
		_ = redisClient.Del(
			context.Background(),
			pkg.RedisKey("session", "user", userID),
			pkg.RedisKey("session", "access", userID),
		).Err()
	})
	persisted, err := userRepository.FindByID(ctx, userID)
	if err != nil {
		t.Fatalf("FindByID(before suspension) error = %v", err)
	}

	sessionRepository := redisSession.NewRepository(redisClient, logger)
	sessions := sessionUseCase.NewUsecase(sessionRepository, logger)
	oldSession := newPasswordChangeIntegrationSession(userID)
	registerCredentialVersionSessionCleanup(t, redisClient, oldSession)
	if err := sessionRepository.Create(ctx, oldSession); err != nil {
		t.Fatalf("create pre-suspension session: %v", err)
	}
	snapshotRead := make(chan struct{})
	continueLogin := make(chan struct{})
	var releaseLogin sync.Once
	release := func() { releaseLogin.Do(func() { close(continueLogin) }) }
	defer release()
	securityStates := &blockingSecurityStateReader{
		SecurityStateReader: userRepository,
		snapshotRead:        snapshotRead,
		continueLogin:       continueLogin,
	}
	jwtManager := authInfra.NewJWTManager("suspension-integration-secret")
	login := authUseCase.NewUsecase(
		userRepository,
		securityStates,
		sessions,
		passwordHasher,
		jwtManager,
		logger,
		authUseCase.AccessTTL(5*time.Minute),
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
			Email:    persisted.Email,
			Password: passwordChangeOldPassword,
			IP:       "127.0.0.1",
			Device:   "concurrent-suspension-login",
		})
		loginDone <- loginResult{output: output, err: loginErr}
	}()

	select {
	case <-snapshotRead:
	case <-ctx.Done():
		t.Fatalf("wait for login authorization snapshot: %v", ctx.Err())
	}

	statusChange := NewUsecase(
		userRepository,
		passwordHasher,
		logger,
		sessionRepository,
		nil,
	)
	if err := statusChange.ChangeStatus(ctx, UpdateStatusInput{
		UserID:    userID,
		ActorID:   uuid.NewString(),
		ActorRole: valueobject.RoleAdmin,
		Status:    valueobject.StatusSuspended,
	}); err != nil {
		t.Fatalf("ChangeStatus(suspended) error = %v", err)
	}
	release()

	var result loginResult
	select {
	case result = <-loginDone:
	case <-ctx.Done():
		t.Fatalf("wait for delayed login: %v", ctx.Err())
	}
	if !errors.Is(result.err, domainErrors.ErrAccountSuspended) {
		t.Fatalf("delayed Login() error = %v, want ErrAccountSuspended", result.err)
	}
	if result.output.AccessToken != "" || result.output.RefreshToken != "" {
		t.Fatalf("delayed Login() returned tokens after suspension: %#v", result.output)
	}

	persisted, err = userRepository.FindByID(ctx, userID)
	if err != nil {
		t.Fatalf("FindByID(after suspension) error = %v", err)
	}
	if persisted.Status != valueobject.StatusSuspended {
		t.Fatalf("persisted status = %s, want suspended", persisted.Status)
	}
	if blocked, err := redisClient.HGet(
		ctx,
		pkg.RedisKey("session", "access", userID),
		"blocked",
	).Result(); err != nil || blocked != "1" {
		t.Fatalf("Redis access block = (%q, %v), want blocked", blocked, err)
	}
	if count, err := redisClient.ZCard(
		ctx,
		pkg.RedisKey("session", "user", userID),
	).Result(); err != nil || count != 0 {
		t.Fatalf("late-login session index size = (%d, %v), want 0", count, err)
	}
	assertDeleteUserRedisKeysAbsent(t, ctx, redisClient, oldSession)
	valid, err := sessions.ValidateSession(ctx, sessionUseCase.ValidateInput{
		UserID:            oldSession.UserID,
		SessionID:         oldSession.ID,
		JTI:               oldSession.CurrentJTI,
		CredentialVersion: oldSession.CredentialVersion,
	})
	if !errors.Is(err, domainErrors.ErrUserAccessBlocked) || valid {
		t.Fatalf(
			"pre-reactivation snapshot validation = (%v, %v), want blocked",
			valid,
			err,
		)
	}

	if err := statusChange.ChangeStatus(ctx, UpdateStatusInput{
		UserID:    userID,
		ActorID:   uuid.NewString(),
		ActorRole: valueobject.RoleAdmin,
		Status:    valueobject.StatusActive,
	}); err != nil {
		t.Fatalf("ChangeStatus(active) error = %v", err)
	}
	persisted, err = userRepository.FindByID(ctx, userID)
	if err != nil {
		t.Fatalf("FindByID(after reactivation) error = %v", err)
	}
	if persisted.Status != valueobject.StatusActive {
		t.Fatalf("reactivated status = %s, want active", persisted.Status)
	}
	accessValues, err := redisClient.HMGet(
		ctx,
		pkg.RedisKey("session", "access", userID),
		"blocked",
		"session_generation",
	).Result()
	if err != nil {
		t.Fatalf("read reactivated access state: %v", err)
	}
	if len(accessValues) != 2 || accessValues[0] != "0" {
		t.Fatalf("reactivated access state = %#v, want unblocked", accessValues)
	}
	generation, err := redisClient.HGet(
		ctx,
		pkg.RedisKey("session", "access", userID),
		"session_generation",
	).Int64()
	if err != nil || generation <= oldSession.SessionGeneration {
		t.Fatalf(
			"reactivated generation = (%d, %v), want greater than old generation %d",
			generation,
			err,
			oldSession.SessionGeneration,
		)
	}
	valid, err = sessions.ValidateSession(ctx, sessionUseCase.ValidateInput{
		UserID:            oldSession.UserID,
		SessionID:         oldSession.ID,
		JTI:               oldSession.CurrentJTI,
		CredentialVersion: oldSession.CredentialVersion,
	})
	if err != nil || valid {
		t.Fatalf("old session after reactivation = (%v, %v), want invalid without error", valid, err)
	}
}

func newCredentialVersionIntegration(
	t *testing.T,
) (context.Context, *postgresUser.Repository, service.PasswordHasher, *redis.Client) {
	t.Helper()
	dsn := os.Getenv("GOAT_POSTGRES_TEST_DSN")
	if dsn == "" || os.Getenv("GOAT_REDIS_ADDR") == "" {
		t.Skip("set GOAT_POSTGRES_TEST_DSN and GOAT_REDIS_ADDR to run credential-version integration tests")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	t.Cleanup(cancel)
	db, err := sqlx.Connect("pgx", dsn)
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
		) ON COMMIT PRESERVE ROWS;
		CREATE TEMP TABLE permissions (
			id UUID PRIMARY KEY,
			role SMALLINT NOT NULL,
			action TEXT NOT NULL,
			created_at TIMESTAMP NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMP NULL,
			CONSTRAINT unique_role_action UNIQUE(role, action)
		) ON COMMIT PRESERVE ROWS;
		INSERT INTO permissions (id, role, action)
		VALUES ('00000000-0000-4000-8000-000000000001', 1, 'user:read')
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
			pkg.RedisKey("session", "access", current.UserID),
		).Err()
	})
}

type blockingSecurityStateReader struct {
	authorization.SecurityStateReader
	snapshotRead  chan struct{}
	continueLogin chan struct{}
	once          sync.Once
}

func (r *blockingSecurityStateReader) GetSecurityState(
	ctx context.Context,
	userID string,
) (authorization.SecurityState, error) {
	state, err := r.SecurityStateReader.GetSecurityState(ctx, userID)
	var waitErr error
	r.once.Do(func() {
		close(r.snapshotRead)
		select {
		case <-r.continueLogin:
		case <-ctx.Done():
			waitErr = ctx.Err()
		}
	})
	if waitErr != nil {
		return authorization.SecurityState{}, waitErr
	}
	return state, err
}
