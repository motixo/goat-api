package user

import (
	"context"
	stdErrors "errors"
	"testing"
	"time"

	"github.com/motixo/goat-api/internal/domain/entity"
	domainErrors "github.com/motixo/goat-api/internal/domain/errors"
	"github.com/motixo/goat-api/internal/domain/repository"
	redisSession "github.com/motixo/goat-api/internal/infra/storage/redis/session"
	"github.com/motixo/goat-api/internal/pkg"
	sessionUseCase "github.com/motixo/goat-api/internal/usecase/session"
	"github.com/redis/go-redis/v9"
)

func TestDeleteUserIntegrationCleansIndexedSessionsBeforeDeletingUser(t *testing.T) {
	tests := []struct {
		name              string
		sessionCount      int
		addInvalidMembers bool
	}{
		{name: "no sessions"},
		{name: "one session", sessionCount: 1},
		{name: "multiple sessions with stale and foreign references", sessionCount: 3, addInvalidMembers: true},
	}

	for _, currentTest := range tests {
		t.Run(currentTest.name, func(t *testing.T) {
			ctx, users, passwordHasher, redisClient := newCredentialVersionIntegration(t)
			logger := discardUserLogger{}
			userID := createCredentialVersionIntegrationUser(t, ctx, users, passwordHasher)
			sessions := redisSession.NewRepository(redisClient, logger)

			owned := makeDeleteUserIntegrationSessions(t, ctx, redisClient, sessions, userID, currentTest.sessionCount)

			var foreignSessionID string
			var foreignJTI string
			if currentTest.addInvalidMembers {
				foreign := newPasswordChangeIntegrationSession("foreign-user-" + pkg.ULIDGenerator())
				foreignSessionID = foreign.ID
				foreignJTI = foreign.CurrentJTI
				registerCredentialVersionSessionCleanup(t, redisClient, foreign)
				if err := sessions.Create(ctx, foreign); err != nil {
					t.Fatalf("create foreign session: %v", err)
				}

				targetIndex := pkg.RedisKey("session", "user", userID)
				staleKey := pkg.RedisKey("session", "id", pkg.ULIDGenerator())
				t.Cleanup(func() { _ = redisClient.Del(context.Background(), targetIndex, staleKey).Err() })
				if err := redisClient.ZAdd(
					ctx,
					targetIndex,
					redis.Z{Score: float64(time.Now().Unix()), Member: staleKey},
					redis.Z{
						Score:  float64(time.Now().Add(time.Second).Unix()),
						Member: pkg.RedisKey("session", "id", foreign.ID),
					},
				).Err(); err != nil {
					t.Fatalf("add stale and foreign index members: %v", err)
				}
			}

			usecase := NewUsecase(users, nil, logger, sessions, nil)
			if err := usecase.DeleteUser(ctx, userID); err != nil {
				t.Fatalf("DeleteUser() error = %v", err)
			}

			if _, err := users.FindByID(ctx, userID); !stdErrors.Is(err, domainErrors.ErrUserNotFound) {
				t.Fatalf("FindByID(deleted user) error = %v, want ErrUserNotFound", err)
			}
			assertDeleteUserRedisKeysAbsent(t, ctx, redisClient, owned...)
			if currentTest.addInvalidMembers {
				assertDeleteUserRedisKeysPresent(t, ctx, redisClient, foreignSessionID, foreignJTI)
				targetIndex := pkg.RedisKey("session", "user", userID)
				if members, err := redisClient.ZCard(ctx, targetIndex).Result(); err != nil || members != 0 {
					t.Fatalf("target session index size = (%d, %v), want (0, nil)", members, err)
				}
			}
		})
	}
}

func TestDeleteUserIntegrationIsIdempotentAtTheSessionBoundary(t *testing.T) {
	ctx, users, passwordHasher, redisClient := newCredentialVersionIntegration(t)
	logger := discardUserLogger{}
	userID := createCredentialVersionIntegrationUser(t, ctx, users, passwordHasher)
	sessions := redisSession.NewRepository(redisClient, logger)
	owned := makeDeleteUserIntegrationSessions(t, ctx, redisClient, sessions, userID, 1)
	usecase := NewUsecase(users, nil, logger, sessions, nil)

	if err := usecase.DeleteUser(ctx, userID); err != nil {
		t.Fatalf("DeleteUser(first) error = %v", err)
	}
	err := usecase.DeleteUser(ctx, userID)
	if !stdErrors.Is(err, domainErrors.ErrUserNotFound) {
		t.Fatalf("DeleteUser(second) error = %v, want ErrUserNotFound", err)
	}
	assertDeleteUserRedisKeysAbsent(t, ctx, redisClient, owned...)
}

func TestDeleteUserIntegrationRedisCleanupFailureLeavesPostgreSQLUntouched(t *testing.T) {
	ctx, users, passwordHasher, operationClient := newCredentialVersionIntegration(t)
	logger := discardUserLogger{}
	userID := createCredentialVersionIntegrationUser(t, ctx, users, passwordHasher)
	sessions := redisSession.NewRepository(operationClient, logger)
	owned := makeDeleteUserIntegrationSessions(t, ctx, operationClient, sessions, userID, 1)

	if err := operationClient.Close(); err != nil {
		t.Fatalf("close operation Redis client: %v", err)
	}
	err := NewUsecase(users, nil, logger, sessions, nil).DeleteUser(ctx, userID)
	if !stdErrors.Is(err, redis.ErrClosed) {
		t.Fatalf("DeleteUser() error = %v, want redis.ErrClosed", err)
	}
	assertDeleteUserIntegrationUserExists(t, ctx, users, userID)

	observer := newCredentialVersionRedisClient(t)
	for _, current := range owned {
		registerCredentialVersionSessionCleanup(t, observer, current)
		assertDeleteUserRedisKeysPresent(t, ctx, observer, current.ID, current.CurrentJTI)
	}
}

func TestDeleteUserIntegrationPostgreSQLFailureOccursAfterCleanup(t *testing.T) {
	ctx, users, passwordHasher, redisClient := newCredentialVersionIntegration(t)
	logger := discardUserLogger{}
	userID := createCredentialVersionIntegrationUser(t, ctx, users, passwordHasher)
	sessions := redisSession.NewRepository(redisClient, logger)
	owned := makeDeleteUserIntegrationSessions(t, ctx, redisClient, sessions, userID, 1)

	deleteErr := stdErrors.New("postgres delete failed")
	failingUsers := &deleteUserFailingRepository{
		UserRepository: users,
		deleteErr:      deleteErr,
	}
	err := NewUsecase(failingUsers, nil, logger, sessions, nil).DeleteUser(ctx, userID)
	if !stdErrors.Is(err, deleteErr) {
		t.Fatalf("DeleteUser() error = %v, want PostgreSQL deletion error", err)
	}

	assertDeleteUserIntegrationUserExists(t, ctx, users, userID)
	assertDeleteUserRedisKeysAbsent(t, ctx, redisClient, owned...)
}

func TestDeleteUserIntegrationBlockPreventsLateSessionAndSnapshotAccess(t *testing.T) {
	ctx, users, passwordHasher, redisClient := newCredentialVersionIntegration(t)
	logger := discardUserLogger{}
	userID := createCredentialVersionIntegrationUser(t, ctx, users, passwordHasher)
	sessions := redisSession.NewRepository(redisClient, logger)
	initial := makeDeleteUserIntegrationSessions(t, ctx, redisClient, sessions, userID, 1)

	deleteReached := make(chan struct{})
	continueDelete := make(chan struct{})
	gatedUsers := &deleteUserGatedRepository{
		UserRepository: users,
		deleteReached:  deleteReached,
		continueDelete: continueDelete,
	}
	deleteDone := make(chan error, 1)
	go func() {
		deleteDone <- NewUsecase(gatedUsers, nil, logger, sessions, nil).
			DeleteUser(ctx, userID)
	}()

	select {
	case <-deleteReached:
	case <-ctx.Done():
		t.Fatalf("wait for PostgreSQL deletion boundary: %v", ctx.Err())
	}
	assertDeleteUserRedisKeysAbsent(t, ctx, redisClient, initial...)

	late := newPasswordChangeIntegrationSession(userID)
	registerCredentialVersionSessionCleanup(t, redisClient, late)
	if err := sessions.Create(ctx, late); !stdErrors.Is(err, domainErrors.ErrUserAccessBlocked) {
		t.Fatalf("create session after access block error = %v, want ErrUserAccessBlocked", err)
	}
	assertDeleteUserRedisKeysAbsent(t, ctx, redisClient, late)

	validation := sessionUseCase.NewUsecase(sessions, logger)
	valid, err := validation.ValidateSession(ctx, sessionUseCase.ValidateInput{
		UserID:            userID,
		SessionID:         initial[0].ID,
		JTI:               initial[0].CurrentJTI,
		CredentialVersion: initial[0].CredentialVersion,
	})
	if !stdErrors.Is(err, domainErrors.ErrUserAccessBlocked) || valid {
		t.Fatalf(
			"ValidateSession(blocked deleted-user session) = (%v, %v), want (false, ErrUserAccessBlocked)",
			valid,
			err,
		)
	}
	close(continueDelete)

	select {
	case err := <-deleteDone:
		if err != nil {
			t.Fatalf("DeleteUser() error = %v", err)
		}
	case <-ctx.Done():
		t.Fatalf("wait for user deletion: %v", ctx.Err())
	}

	if blocked, err := redisClient.HGet(
		ctx,
		pkg.RedisKey("session", "access", userID),
		"blocked",
	).Result(); err != nil || blocked != "1" {
		t.Fatalf("deleted-user access state = (%q, %v), want blocked", blocked, err)
	}

	afterDelete := newPasswordChangeIntegrationSession(userID)
	registerCredentialVersionSessionCleanup(t, redisClient, afterDelete)
	if err := sessions.Create(ctx, afterDelete); !stdErrors.Is(err, domainErrors.ErrUserAccessBlocked) {
		t.Fatalf("create session after PostgreSQL deletion error = %v, want ErrUserAccessBlocked", err)
	}
	assertDeleteUserRedisKeysAbsent(t, ctx, redisClient, afterDelete)
}

func makeDeleteUserIntegrationSessions(
	t *testing.T,
	ctx context.Context,
	client *redis.Client,
	sessions repository.SessionRepository,
	userID string,
	count int,
) []*entity.Session {
	t.Helper()
	created := make([]*entity.Session, 0, count)
	for range count {
		current := newPasswordChangeIntegrationSession(userID)
		registerCredentialVersionSessionCleanup(t, client, current)
		if err := sessions.Create(ctx, current); err != nil {
			t.Fatalf("create session: %v", err)
		}
		created = append(created, current)
	}
	return created
}

func assertDeleteUserRedisKeysAbsent(
	t *testing.T,
	ctx context.Context,
	client *redis.Client,
	sessions ...*entity.Session,
) {
	t.Helper()
	for _, current := range sessions {
		exists, err := client.Exists(
			ctx,
			pkg.RedisKey("session", "id", current.ID),
			pkg.RedisKey("session", "jti", current.CurrentJTI),
		).Result()
		if err != nil || exists != 0 {
			t.Fatalf("session %q Redis key existence = (%d, %v), want (0, nil)", current.ID, exists, err)
		}
	}
}

func assertDeleteUserRedisKeysPresent(
	t *testing.T,
	ctx context.Context,
	client *redis.Client,
	sessionID string,
	jti string,
) {
	t.Helper()
	exists, err := client.Exists(
		ctx,
		pkg.RedisKey("session", "id", sessionID),
		pkg.RedisKey("session", "jti", jti),
	).Result()
	if err != nil || exists != 2 {
		t.Fatalf("session %q Redis key existence = (%d, %v), want (2, nil)", sessionID, exists, err)
	}
}

func assertDeleteUserIntegrationUserExists(
	t *testing.T,
	ctx context.Context,
	users repository.UserRepository,
	userID string,
) {
	t.Helper()
	user, err := users.FindByID(ctx, userID)
	if err != nil {
		t.Fatalf("FindByID(retained user) error = %v", err)
	}
	if user == nil {
		t.Fatal("FindByID(retained user) returned nil")
	}
}

type deleteUserFailingRepository struct {
	repository.UserRepository
	deleteErr error
}

func (r *deleteUserFailingRepository) Delete(context.Context, string) error {
	return r.deleteErr
}

type deleteUserGatedRepository struct {
	repository.UserRepository
	deleteReached  chan struct{}
	continueDelete chan struct{}
}

func (r *deleteUserGatedRepository) Delete(ctx context.Context, userID string) error {
	close(r.deleteReached)
	select {
	case <-r.continueDelete:
		return r.UserRepository.Delete(ctx, userID)
	case <-ctx.Done():
		return ctx.Err()
	}
}
