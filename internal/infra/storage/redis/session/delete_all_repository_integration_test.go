package session

import (
	"context"
	"errors"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/motixo/goat-api/internal/domain/entity"
	redisStorage "github.com/motixo/goat-api/internal/infra/storage/redis"
	"github.com/motixo/goat-api/internal/pkg"
	"github.com/redis/go-redis/v9"
)

func TestDeleteAllByUserHandlesIndexedSessionStates(t *testing.T) {
	ctx, client, repository := newDeleteAllIntegrationRepository(t)

	t.Run("deletes every owned session and is idempotent", func(t *testing.T) {
		userID := "integration-user-" + pkg.ULIDGenerator()
		sessions := []*entity.Session{
			newIntegrationSession(userID, time.Hour),
			newIntegrationSession(userID, time.Hour),
			newIntegrationSession(userID, time.Hour),
		}
		registerRedisSessionCleanup(t, client, sessions...)
		createRedisSessions(t, ctx, repository, sessions...)

		for attempt := 0; attempt < 2; attempt++ {
			if err := repository.DeleteAllByUser(ctx, userID); err != nil {
				t.Fatalf("DeleteAllByUser() attempt %d error = %v", attempt+1, err)
			}
		}
		for _, current := range sessions {
			assertRedisSessionAbsent(t, ctx, client, current)
		}
	})

	t.Run("no sessions is successful", func(t *testing.T) {
		userID := "integration-user-" + pkg.ULIDGenerator()
		if err := repository.DeleteAllByUser(ctx, userID); err != nil {
			t.Fatalf("DeleteAllByUser() error = %v", err)
		}
	})

	t.Run("removes stale index members", func(t *testing.T) {
		userID := "integration-user-" + pkg.ULIDGenerator()
		userKey := pkg.RedisKey("session", "user", userID)
		staleSessionKey := pkg.RedisKey("session", "id", pkg.ULIDGenerator())
		t.Cleanup(func() { _ = client.Del(context.Background(), userKey).Err() })
		if err := client.ZAdd(ctx, userKey, redis.Z{
			Score:  float64(time.Now().Unix()),
			Member: staleSessionKey,
		}).Err(); err != nil {
			t.Fatalf("add stale index member: %v", err)
		}

		if err := repository.DeleteAllByUser(ctx, userID); err != nil {
			t.Fatalf("DeleteAllByUser() error = %v", err)
		}
		assertRedisIndexMemberAbsent(t, ctx, client, userKey, staleSessionKey)
	})

	t.Run("removes references to expired session hashes", func(t *testing.T) {
		userID := "integration-user-" + pkg.ULIDGenerator()
		expiring := newIntegrationSession(userID, time.Second)
		registerRedisSessionCleanup(t, client, expiring)
		createRedisSessions(t, ctx, repository, expiring)

		userKey := pkg.RedisKey("session", "user", userID)
		if err := client.Expire(ctx, userKey, time.Hour).Err(); err != nil {
			t.Fatalf("extend user index expiry: %v", err)
		}
		waitForRedisKeysToExpire(
			t,
			ctx,
			client,
			pkg.RedisKey("session", "id", expiring.ID),
			pkg.RedisKey("session", "jti", expiring.CurrentJTI),
		)
		sessionKey := pkg.RedisKey("session", "id", expiring.ID)
		if err := client.ZScore(ctx, userKey, sessionKey).Err(); err != nil {
			t.Fatalf("expired session was not retained as an index reference: %v", err)
		}

		if err := repository.DeleteAllByUser(ctx, userID); err != nil {
			t.Fatalf("DeleteAllByUser() error = %v", err)
		}
		assertRedisIndexMemberAbsent(t, ctx, client, userKey, sessionKey)
	})

	t.Run("removes incomplete owned records and preserves unowned records", func(t *testing.T) {
		userID := "integration-user-" + pkg.ULIDGenerator()
		userKey := pkg.RedisKey("session", "user", userID)
		ownedIncompleteKey := pkg.RedisKey("session", "id", pkg.ULIDGenerator())
		unownedIncompleteKey := pkg.RedisKey("session", "id", pkg.ULIDGenerator())
		wrongTypeKey := pkg.RedisKey("session", "id", pkg.ULIDGenerator())
		unexpectedOwnedKey := "corrupt:session:" + pkg.ULIDGenerator()
		t.Cleanup(func() {
			_ = client.Del(
				context.Background(),
				userKey,
				ownedIncompleteKey,
				unownedIncompleteKey,
				wrongTypeKey,
				unexpectedOwnedKey,
			).Err()
		})
		if err := client.HSet(ctx, ownedIncompleteKey, "user_id", userID).Err(); err != nil {
			t.Fatalf("create incomplete owned hash: %v", err)
		}
		if err := client.HSet(ctx, unownedIncompleteKey, "id", pkg.ULIDGenerator()).Err(); err != nil {
			t.Fatalf("create incomplete unowned hash: %v", err)
		}
		if err := client.Set(ctx, wrongTypeKey, "corrupt", time.Hour).Err(); err != nil {
			t.Fatalf("create wrong-type session record: %v", err)
		}
		if err := client.HSet(ctx, unexpectedOwnedKey, "user_id", userID).Err(); err != nil {
			t.Fatalf("create unexpected owned hash: %v", err)
		}
		if err := client.ZAdd(
			ctx,
			userKey,
			redis.Z{Score: 4, Member: ownedIncompleteKey},
			redis.Z{Score: 3, Member: unownedIncompleteKey},
			redis.Z{Score: 2, Member: wrongTypeKey},
			redis.Z{Score: 1, Member: unexpectedOwnedKey},
		).Err(); err != nil {
			t.Fatalf("index incomplete records: %v", err)
		}

		if err := repository.DeleteAllByUser(ctx, userID); err != nil {
			t.Fatalf("DeleteAllByUser() error = %v", err)
		}
		if exists, err := client.Exists(ctx, ownedIncompleteKey).Result(); err != nil || exists != 0 {
			t.Fatalf("owned incomplete hash existence = (%d, %v), want (0, nil)", exists, err)
		}
		for _, preservedKey := range []string{
			unownedIncompleteKey,
			wrongTypeKey,
			unexpectedOwnedKey,
		} {
			if exists, err := client.Exists(ctx, preservedKey).Result(); err != nil || exists != 1 {
				t.Fatalf("unowned record %q existence = (%d, %v), want (1, nil)", preservedKey, exists, err)
			}
			assertRedisIndexMemberAbsent(t, ctx, client, userKey, preservedKey)
		}
	})

	t.Run("foreign index references do not affect foreign sessions", func(t *testing.T) {
		targetUserID := "integration-user-" + pkg.ULIDGenerator()
		foreign := newIntegrationSession("integration-user-"+pkg.ULIDGenerator(), time.Hour)
		registerRedisSessionCleanup(t, client, foreign)
		createRedisSessions(t, ctx, repository, foreign)

		targetUserKey := pkg.RedisKey("session", "user", targetUserID)
		foreignSessionKey := pkg.RedisKey("session", "id", foreign.ID)
		t.Cleanup(func() { _ = client.Del(context.Background(), targetUserKey).Err() })
		if err := client.ZAdd(ctx, targetUserKey, redis.Z{
			Score:  float64(time.Now().Unix()),
			Member: foreignSessionKey,
		}).Err(); err != nil {
			t.Fatalf("add foreign session reference: %v", err)
		}

		if err := repository.DeleteAllByUser(ctx, targetUserID); err != nil {
			t.Fatalf("DeleteAllByUser() error = %v", err)
		}
		assertRedisSessionPresent(t, ctx, client, foreign)
		assertRedisIndexMemberAbsent(t, ctx, client, targetUserKey, foreignSessionKey)
	})

	t.Run("corrupt owned JTI reference cannot delete a foreign JTI", func(t *testing.T) {
		targetUserID := "integration-user-" + pkg.ULIDGenerator()
		owned := newIntegrationSession(targetUserID, time.Hour)
		foreign := newIntegrationSession("integration-user-"+pkg.ULIDGenerator(), time.Hour)
		registerRedisSessionCleanup(t, client, owned, foreign)
		createRedisSessions(t, ctx, repository, owned, foreign)

		ownedSessionKey := pkg.RedisKey("session", "id", owned.ID)
		if err := client.Del(ctx, pkg.RedisKey("session", "jti", owned.CurrentJTI)).Err(); err != nil {
			t.Fatalf("remove original owned JTI: %v", err)
		}
		if err := client.HSet(ctx, ownedSessionKey, "current_jti", foreign.CurrentJTI).Err(); err != nil {
			t.Fatalf("corrupt owned current JTI: %v", err)
		}

		if err := repository.DeleteAllByUser(ctx, targetUserID); err != nil {
			t.Fatalf("DeleteAllByUser() error = %v", err)
		}
		if exists, err := client.Exists(ctx, ownedSessionKey).Result(); err != nil || exists != 0 {
			t.Fatalf("owned hash existence = (%d, %v), want (0, nil)", exists, err)
		}
		assertRedisIndexMemberAbsent(
			t,
			ctx,
			client,
			pkg.RedisKey("session", "user", targetUserID),
			ownedSessionKey,
		)
		assertRedisSessionPresent(t, ctx, client, foreign)
	})
}

func TestDeleteAllByUserIsOneAtomicRedisExecution(t *testing.T) {
	ctx, setupClient, setupRepository := newDeleteAllIntegrationRepository(t)
	userID := "integration-user-" + pkg.ULIDGenerator()
	sessions := []*entity.Session{
		newIntegrationSession(userID, time.Hour),
		newIntegrationSession(userID, time.Hour),
	}
	registerRedisSessionCleanup(t, setupClient, sessions...)
	createRedisSessions(t, ctx, setupRepository, sessions...)

	counter := &redisCommandCounter{}
	client := redis.NewClient(&redis.Options{
		Addr:                  os.Getenv("GOAT_REDIS_ADDR"),
		ContextTimeoutEnabled: true,
	})
	client.AddHook(counter)
	t.Cleanup(func() { _ = client.Close() })
	if err := client.Ping(ctx).Err(); err != nil {
		t.Fatalf("ping counting Redis client: %v", err)
	}
	if err := redisStorage.GetScript("delete_other_sessions").Load(ctx, client).Err(); err != nil {
		t.Fatalf("load owner-index deletion script: %v", err)
	}
	counter.Reset()

	repository := &Repository{client: client}
	if err := repository.DeleteAllByUser(ctx, userID); err != nil {
		t.Fatalf("DeleteAllByUser() error = %v", err)
	}
	commands := counter.Commands()
	if len(commands) != 1 || commands[0] != "evalsha" {
		t.Fatalf("Redis commands = %v, want exactly one evalsha", commands)
	}
	for _, current := range sessions {
		assertRedisSessionAbsent(t, ctx, setupClient, current)
	}
}

func TestDeleteAllByUserConcurrentCreationHasNoPartialSessionState(t *testing.T) {
	ctx, client, repository := newDeleteAllIntegrationRepository(t)
	userID := "integration-user-" + pkg.ULIDGenerator()
	concurrent := newIntegrationSession(userID, time.Hour)
	registerRedisSessionCleanup(t, client, concurrent)

	start := make(chan struct{})
	var wait sync.WaitGroup
	wait.Add(2)
	var createErr error
	var deleteErr error
	go func() {
		defer wait.Done()
		<-start
		createErr = repository.Create(ctx, concurrent)
	}()
	go func() {
		defer wait.Done()
		<-start
		deleteErr = repository.DeleteAllByUser(ctx, userID)
	}()
	close(start)
	wait.Wait()

	if createErr != nil {
		t.Fatalf("Create() error = %v", createErr)
	}
	if deleteErr != nil {
		t.Fatalf("DeleteAllByUser() error = %v", deleteErr)
	}
	assertRedisSessionStateIsComplete(t, ctx, client, concurrent)
}

func TestDeleteAllByUserTimeoutAndClientFailureDoNotStartMutation(t *testing.T) {
	ctx, observer, setupRepository := newDeleteAllIntegrationRepository(t)
	current := newIntegrationSession("integration-user-"+pkg.ULIDGenerator(), time.Hour)
	registerRedisSessionCleanup(t, observer, current)
	createRedisSessions(t, ctx, setupRepository, current)

	operationClient := redis.NewClient(&redis.Options{
		Addr:                  os.Getenv("GOAT_REDIS_ADDR"),
		ContextTimeoutEnabled: true,
	})
	repository := &Repository{client: operationClient}

	timeoutCtx, cancel := context.WithTimeout(ctx, time.Nanosecond)
	<-timeoutCtx.Done()
	err := repository.DeleteAllByUser(timeoutCtx, current.UserID)
	cancel()
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("DeleteAllByUser(expired context) error = %v, want context deadline exceeded", err)
	}
	assertRedisSessionPresent(t, ctx, observer, current)

	if err := operationClient.Close(); err != nil {
		t.Fatalf("close operation Redis client: %v", err)
	}
	err = repository.DeleteAllByUser(ctx, current.UserID)
	if !errors.Is(err, redis.ErrClosed) {
		t.Fatalf("DeleteAllByUser(closed client) error = %v, want redis.ErrClosed", err)
	}
	assertRedisSessionPresent(t, ctx, observer, current)
}

func newDeleteAllIntegrationRepository(t *testing.T) (context.Context, *redis.Client, *Repository) {
	t.Helper()
	address := os.Getenv("GOAT_REDIS_ADDR")
	if address == "" {
		t.Skip("set GOAT_REDIS_ADDR to run Redis integration tests")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	t.Cleanup(cancel)
	client := redis.NewClient(&redis.Options{
		Addr:                  address,
		ContextTimeoutEnabled: true,
	})
	t.Cleanup(func() { _ = client.Close() })
	if err := client.Ping(ctx).Err(); err != nil {
		t.Fatalf("ping Redis: %v", err)
	}
	return ctx, client, &Repository{client: client}
}
