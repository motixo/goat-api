package user

import (
	"context"
	stdErrors "errors"
	"testing"
	"time"

	"github.com/motixo/goat-api/internal/domain/entity"
	domainErrors "github.com/motixo/goat-api/internal/domain/errors"
	"github.com/motixo/goat-api/internal/domain/repository"
	usercache "github.com/motixo/goat-api/internal/infra/cache/user"
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
			cache := usercache.NewCachedRepository(users, usercache.NewCache(redisClient), logger)

			owned := makeDeleteUserIntegrationSessions(t, ctx, redisClient, sessions, userID, currentTest.sessionCount)
			cacheKey := pkg.RedisKey("user", "id", userID)
			t.Cleanup(func() { _ = redisClient.Del(context.Background(), cacheKey).Err() })
			if _, err := cache.GetUserRole(ctx, userID); err != nil {
				t.Fatalf("prime authorization cache: %v", err)
			}

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

			usecase := NewUsecase(users, nil, logger, sessions, cache, nil, nil)
			if err := usecase.DeleteUser(ctx, userID); err != nil {
				t.Fatalf("DeleteUser() error = %v", err)
			}

			if _, err := users.FindByID(ctx, userID); !stdErrors.Is(err, domainErrors.ErrUserNotFound) {
				t.Fatalf("FindByID(deleted user) error = %v, want ErrUserNotFound", err)
			}
			assertDeleteUserRedisKeysAbsent(t, ctx, redisClient, owned...)
			if exists, err := redisClient.Exists(ctx, cacheKey).Result(); err != nil || exists != 0 {
				t.Fatalf("authorization cache existence = (%d, %v), want (0, nil)", exists, err)
			}
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
	cache := usercache.NewCachedRepository(users, usercache.NewCache(redisClient), logger)
	owned := makeDeleteUserIntegrationSessions(t, ctx, redisClient, sessions, userID, 1)
	usecase := NewUsecase(users, nil, logger, sessions, cache, nil, nil)

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
	cache := usercache.NewCachedRepository(users, usercache.NewCache(operationClient), logger)

	if err := operationClient.Close(); err != nil {
		t.Fatalf("close operation Redis client: %v", err)
	}
	err := NewUsecase(users, nil, logger, sessions, cache, nil, nil).DeleteUser(ctx, userID)
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

func TestDeleteUserIntegrationCacheFailureLeavesPostgreSQLUntouched(t *testing.T) {
	ctx, users, passwordHasher, redisClient := newCredentialVersionIntegration(t)
	logger := discardUserLogger{}
	userID := createCredentialVersionIntegrationUser(t, ctx, users, passwordHasher)
	sessions := redisSession.NewRepository(redisClient, logger)
	owned := makeDeleteUserIntegrationSessions(t, ctx, redisClient, sessions, userID, 1)

	workingCache := usercache.NewCachedRepository(users, usercache.NewCache(redisClient), logger)
	cacheKey := pkg.RedisKey("user", "id", userID)
	t.Cleanup(func() { _ = redisClient.Del(context.Background(), cacheKey).Err() })
	if _, err := workingCache.GetUserRole(ctx, userID); err != nil {
		t.Fatalf("prime authorization cache: %v", err)
	}

	failingCacheClient := newCredentialVersionRedisClient(t)
	if err := failingCacheClient.Close(); err != nil {
		t.Fatalf("close cache Redis client: %v", err)
	}
	failingCache := usercache.NewCachedRepository(users, usercache.NewCache(failingCacheClient), logger)
	err := NewUsecase(users, nil, logger, sessions, failingCache, nil, nil).DeleteUser(ctx, userID)
	if !stdErrors.Is(err, redis.ErrClosed) {
		t.Fatalf("DeleteUser() error = %v, want redis.ErrClosed", err)
	}

	assertDeleteUserIntegrationUserExists(t, ctx, users, userID)
	assertDeleteUserRedisKeysAbsent(t, ctx, redisClient, owned...)
	if exists, err := redisClient.Exists(ctx, cacheKey).Result(); err != nil || exists != 1 {
		t.Fatalf("authorization cache existence = (%d, %v), want (1, nil)", exists, err)
	}
}

func TestDeleteUserIntegrationPostgreSQLFailureOccursAfterCleanup(t *testing.T) {
	ctx, users, passwordHasher, redisClient := newCredentialVersionIntegration(t)
	logger := discardUserLogger{}
	userID := createCredentialVersionIntegrationUser(t, ctx, users, passwordHasher)
	sessions := redisSession.NewRepository(redisClient, logger)
	owned := makeDeleteUserIntegrationSessions(t, ctx, redisClient, sessions, userID, 1)
	cache := usercache.NewCachedRepository(users, usercache.NewCache(redisClient), logger)
	cacheKey := pkg.RedisKey("user", "id", userID)
	t.Cleanup(func() { _ = redisClient.Del(context.Background(), cacheKey).Err() })
	if _, err := cache.GetUserRole(ctx, userID); err != nil {
		t.Fatalf("prime authorization cache: %v", err)
	}

	deleteErr := stdErrors.New("postgres delete failed")
	failingUsers := &deleteUserFailingRepository{
		UserRepository: users,
		deleteErr:      deleteErr,
	}
	err := NewUsecase(failingUsers, nil, logger, sessions, cache, nil, nil).DeleteUser(ctx, userID)
	if !stdErrors.Is(err, deleteErr) {
		t.Fatalf("DeleteUser() error = %v, want PostgreSQL deletion error", err)
	}

	assertDeleteUserIntegrationUserExists(t, ctx, users, userID)
	assertDeleteUserRedisKeysAbsent(t, ctx, redisClient, owned...)
	if exists, err := redisClient.Exists(ctx, cacheKey).Result(); err != nil || exists != 0 {
		t.Fatalf("authorization cache existence = (%d, %v), want (0, nil)", exists, err)
	}
}

func TestDeleteUserIntegrationLateSessionIsUnusableAfterAuthoritativeDeletion(t *testing.T) {
	ctx, users, passwordHasher, redisClient := newCredentialVersionIntegration(t)
	logger := discardUserLogger{}
	userID := createCredentialVersionIntegrationUser(t, ctx, users, passwordHasher)
	sessions := redisSession.NewRepository(redisClient, logger)
	initial := makeDeleteUserIntegrationSessions(t, ctx, redisClient, sessions, userID, 1)
	cache := usercache.NewCachedRepository(users, usercache.NewCache(redisClient), logger)

	deleteReached := make(chan struct{})
	continueDelete := make(chan struct{})
	gatedUsers := &deleteUserGatedRepository{
		UserRepository: users,
		deleteReached:  deleteReached,
		continueDelete: continueDelete,
	}
	deleteDone := make(chan error, 1)
	go func() {
		deleteDone <- NewUsecase(gatedUsers, nil, logger, sessions, cache, nil, nil).
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
	if err := sessions.Create(ctx, late); err != nil {
		t.Fatalf("create session after atomic cleanup: %v", err)
	}
	assertDeleteUserRedisKeysPresent(t, ctx, redisClient, late.ID, late.CurrentJTI)
	close(continueDelete)

	select {
	case err := <-deleteDone:
		if err != nil {
			t.Fatalf("DeleteUser() error = %v", err)
		}
	case <-ctx.Done():
		t.Fatalf("wait for user deletion: %v", ctx.Err())
	}

	validation := sessionUseCase.NewUsecase(sessions, users, logger)
	valid, err := validation.ValidateSession(ctx, sessionUseCase.ValidateInput{
		UserID:    userID,
		SessionID: late.ID,
		JTI:       late.CurrentJTI,
	})
	if err != nil {
		t.Fatalf("ValidateSession(late session) error = %v", err)
	}
	if valid {
		t.Fatal("late Redis session remained usable after authoritative user deletion")
	}
	assertDeleteUserRedisKeysPresent(t, ctx, redisClient, late.ID, late.CurrentJTI)
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
