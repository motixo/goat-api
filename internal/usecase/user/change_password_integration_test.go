package user

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/jmoiron/sqlx"
	"github.com/motixo/goat-api/internal/config"
	"github.com/motixo/goat-api/internal/domain/entity"
	domainErrors "github.com/motixo/goat-api/internal/domain/errors"
	"github.com/motixo/goat-api/internal/domain/valueobject"
	authInfra "github.com/motixo/goat-api/internal/infra/auth"
	postgresUser "github.com/motixo/goat-api/internal/infra/database/postgres/user"
	redisSession "github.com/motixo/goat-api/internal/infra/storage/redis/session"
	"github.com/motixo/goat-api/internal/pkg"
	"github.com/redis/go-redis/v9"
)

func TestChangePasswordIntegrationCommitsCredentialsAndRevokesSessions(t *testing.T) {
	dsn := os.Getenv("GOAT_POSTGRES_TEST_DSN")
	redisAddress := os.Getenv("GOAT_REDIS_ADDR")
	if dsn == "" || redisAddress == "" {
		t.Skip("set GOAT_POSTGRES_TEST_DSN and GOAT_REDIS_ADDR to run cross-adapter integration tests")
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
		) ON COMMIT PRESERVE ROWS
	`
	if _, err := db.ExecContext(ctx, schema); err != nil {
		t.Fatalf("create temporary users table: %v", err)
	}

	redisClient := redis.NewClient(&redis.Options{Addr: redisAddress})
	t.Cleanup(func() {
		if err := redisClient.Close(); err != nil {
			t.Errorf("close Redis: %v", err)
		}
	})
	if err := redisClient.Ping(ctx).Err(); err != nil {
		t.Fatalf("ping Redis: %v", err)
	}

	logger := passwordChangeLogger{}
	passwordHasher := authInfra.NewPasswordService(&config.Config{PasswordPepper: "integration-pepper"})
	oldHash, err := passwordHasher.Hash(ctx, passwordChangeOldPassword)
	if err != nil {
		t.Fatalf("hash existing password: %v", err)
	}
	userID := uuid.NewString()
	userRepository := postgresUser.NewRepository(db)
	if err := userRepository.Create(ctx, &entity.User{
		ID:                userID,
		Email:             "password-change-" + userID + "@example.com",
		Password:          oldHash,
		Status:            valueobject.StatusActive,
		Role:              valueobject.RoleClient,
		CredentialVersion: entity.InitialCredentialVersion,
		CreatedAt:         time.Now().UTC(),
	}); err != nil {
		t.Fatalf("create integration user: %v", err)
	}

	sessionRepository := redisSession.NewRepository(redisClient, logger)
	sessions := []*entity.Session{
		newPasswordChangeIntegrationSession(userID),
		newPasswordChangeIntegrationSession(userID),
	}
	redisKeys := []string{pkg.RedisKey("session", "user", userID)}
	for _, current := range sessions {
		redisKeys = append(redisKeys,
			pkg.RedisKey("session", "id", current.ID),
			pkg.RedisKey("session", "jti", current.CurrentJTI),
		)
	}
	if err := redisClient.Del(ctx, redisKeys...).Err(); err != nil {
		t.Fatalf("clear Redis integration keys before test: %v", err)
	}
	t.Cleanup(func() {
		_ = redisClient.Del(context.Background(), redisKeys...).Err()
	})
	for _, current := range sessions {
		if err := sessionRepository.Create(ctx, current); err != nil {
			t.Fatalf("create integration session %q: %v", current.ID, err)
		}
	}

	usecase := NewUsecase(
		userRepository,
		passwordHasher,
		logger,
		sessionRepository,
		nil,
	)
	err = usecase.ChangePassword(ctx, UpdatePassInput{
		UserID:      userID,
		OldPassword: passwordChangeOldPassword,
		NewPassword: passwordChangeNewPassword,
	})
	if err != nil {
		t.Fatalf("ChangePassword() error = %v", err)
	}

	persisted, err := userRepository.FindByID(ctx, userID)
	if err != nil {
		t.Fatalf("find changed user: %v", err)
	}
	if passwordHasher.Verify(ctx, passwordChangeOldPassword, persisted.Password) {
		t.Fatal("old password still verifies after successful password change")
	}
	if !passwordHasher.Verify(ctx, passwordChangeNewPassword, persisted.Password) {
		t.Fatal("new password does not verify after successful password change")
	}
	if persisted.CredentialVersion != entity.InitialCredentialVersion+1 {
		t.Fatalf(
			"credential version after password change = %d, want %d",
			persisted.CredentialVersion,
			entity.InitialCredentialVersion+1,
		)
	}
	err = usecase.ChangePassword(ctx, UpdatePassInput{
		UserID:      userID,
		OldPassword: passwordChangeOldPassword,
		NewPassword: passwordChangeNewPassword,
	})
	if !errors.Is(err, domainErrors.ErrInvalidPassword) {
		t.Fatalf("repeated ChangePassword() error = %v, want ErrInvalidPassword", err)
	}
	versionAfterRepeat, err := userRepository.GetCredentialVersion(ctx, userID)
	if err != nil {
		t.Fatalf("get credential version after repeated request: %v", err)
	}
	if versionAfterRepeat != persisted.CredentialVersion {
		t.Fatalf(
			"credential version after repeated request = %d, want unchanged %d",
			versionAfterRepeat,
			persisted.CredentialVersion,
		)
	}

	listed, total, err := sessionRepository.ListByUser(ctx, userID, 0, 0)
	if err != nil {
		t.Fatalf("list sessions after password change: %v", err)
	}
	if len(listed) != 0 || total != 0 {
		t.Fatalf("sessions after password change = (%d, %d), want (0, 0)", len(listed), total)
	}
	for _, current := range sessions {
		exists, err := redisClient.Exists(
			ctx,
			pkg.RedisKey("session", "id", current.ID),
			pkg.RedisKey("session", "jti", current.CurrentJTI),
		).Result()
		if err != nil {
			t.Fatalf("check revoked session %q: %v", current.ID, err)
		}
		if exists != 0 {
			t.Fatalf("session %q retained %d Redis keys, want 0", current.ID, exists)
		}
	}
}

func newPasswordChangeIntegrationSession(userID string) *entity.Session {
	now := time.Now().UTC()
	return &entity.Session{
		ID:                pkg.ULIDGenerator(),
		UserID:            userID,
		CredentialVersion: entity.InitialCredentialVersion,
		CurrentJTI:        pkg.ULIDGenerator(),
		Device:            "integration-device",
		IP:                "127.0.0.1",
		CreatedAt:         now,
		UpdatedAt:         now,
		ExpiresAt:         now.Add(time.Hour),
		JTITTLSeconds:     int64(time.Hour.Seconds()),
		SessionTTLSeconds: int64(time.Hour.Seconds()),
	}
}
