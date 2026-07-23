package session

import (
	"context"
	"os"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/motixo/goat-api/internal/domain/entity"
	"github.com/motixo/goat-api/internal/pkg"
	"github.com/redis/go-redis/v9"
)

func TestCredentialVersionRoundTripAndRotation(t *testing.T) {
	address := os.Getenv("GOAT_REDIS_ADDR")
	if address == "" {
		t.Skip("set GOAT_REDIS_ADDR to run Redis integration tests")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	client := redis.NewClient(&redis.Options{Addr: address})
	t.Cleanup(func() { _ = client.Close() })
	if err := client.Ping(ctx).Err(); err != nil {
		t.Fatalf("ping Redis: %v", err)
	}

	repository := &Repository{client: client}
	current := newIntegrationSession("integration-user-"+pkg.ULIDGenerator(), time.Hour)
	current.CredentialVersion = 7
	registerRedisSessionCleanup(t, client, current)
	if err := repository.Create(ctx, current); err != nil {
		t.Fatalf("create session: %v", err)
	}

	sessionKey := pkg.RedisKey("session", "id", current.ID)
	storedVersion, err := client.HGet(ctx, sessionKey, "credential_version").Result()
	if err != nil {
		t.Fatalf("read stored credential version: %v", err)
	}
	if storedVersion != strconv.FormatInt(current.CredentialVersion, 10) {
		t.Fatalf("stored credential version = %q, want %d", storedVersion, current.CredentialVersion)
	}

	found, err := repository.FindByJTI(ctx, current.CurrentJTI)
	if err != nil {
		t.Fatalf("FindByJTI() error = %v", err)
	}
	if found == nil ||
		found.ID != current.ID ||
		found.UserID != current.UserID ||
		found.CurrentJTI != current.CurrentJTI ||
		found.CredentialVersion != current.CredentialVersion {
		t.Fatalf("FindByJTI() = %#v, want identity and credential version from %#v", found, current)
	}

	rejectedJTI := pkg.ULIDGenerator()
	t.Cleanup(func() {
		_ = client.Del(context.Background(), pkg.RedisKey("session", "jti", rejectedJTI)).Err()
	})
	_, err = repository.RotateJTI(
		ctx,
		current.CurrentJTI,
		rejectedJTI,
		current.UserID,
		current.CredentialVersion+1,
		current.IP,
		current.Device,
		time.Now().UTC().Add(time.Hour),
		int64(time.Hour.Seconds()),
		int64(time.Hour.Seconds()),
	)
	if err == nil {
		t.Fatal("RotateJTI(wrong credential version) error = nil")
	}
	assertRedisSessionPresent(t, ctx, client, current)
	if exists, err := client.Exists(ctx, pkg.RedisKey("session", "jti", rejectedJTI)).Result(); err != nil {
		t.Fatalf("check rejected JTI: %v", err)
	} else if exists != 0 {
		t.Fatal("rejected rotation created the new JTI")
	}

	newJTI := pkg.ULIDGenerator()
	t.Cleanup(func() {
		_ = client.Del(context.Background(), pkg.RedisKey("session", "jti", newJTI)).Err()
	})
	rotatedID, err := repository.RotateJTI(
		ctx,
		current.CurrentJTI,
		newJTI,
		current.UserID,
		current.CredentialVersion,
		current.IP,
		current.Device,
		time.Now().UTC().Add(time.Hour),
		int64(time.Hour.Seconds()),
		int64(time.Hour.Seconds()),
	)
	if err != nil {
		t.Fatalf("RotateJTI() error = %v", err)
	}
	if rotatedID != current.ID {
		t.Fatalf("rotated session ID = %q, want %q", rotatedID, current.ID)
	}
	rotated, err := repository.FindByJTI(ctx, newJTI)
	if err != nil {
		t.Fatalf("FindByJTI(rotated) error = %v", err)
	}
	if rotated == nil || rotated.CredentialVersion != current.CredentialVersion {
		t.Fatalf("rotated session = %#v, want credential version %d", rotated, current.CredentialVersion)
	}
	old, err := repository.FindByJTI(ctx, current.CurrentJTI)
	if err != nil {
		t.Fatalf("FindByJTI(old) error = %v", err)
	}
	if old != nil {
		t.Fatalf("old JTI still resolves to %#v", old)
	}
}

func TestListByUserFiltersBeforePagination(t *testing.T) {
	address := os.Getenv("GOAT_REDIS_ADDR")
	if address == "" {
		t.Skip("set GOAT_REDIS_ADDR to run Redis integration tests")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	client := redis.NewClient(&redis.Options{Addr: address})
	t.Cleanup(func() { _ = client.Close() })
	if err := client.Ping(ctx).Err(); err != nil {
		t.Fatalf("ping Redis: %v", err)
	}

	userID := "integration-user-" + pkg.ULIDGenerator()
	newest := newIntegrationSession(userID, time.Hour)
	oldest := newIntegrationSession(userID, time.Hour)
	registerRedisSessionCleanup(t, client, newest, oldest)
	repository := &Repository{client: client}
	createRedisSessions(t, ctx, repository, newest, oldest)

	userKey := pkg.RedisKey("session", "user", userID)
	staleSessionKey := pkg.RedisKey("session", "id", pkg.ULIDGenerator())
	if err := client.ZAdd(ctx, userKey,
		redis.Z{Score: 30, Member: staleSessionKey},
		redis.Z{Score: 20, Member: pkg.RedisKey("session", "id", newest.ID)},
		redis.Z{Score: 10, Member: pkg.RedisKey("session", "id", oldest.ID)},
	).Err(); err != nil {
		t.Fatalf("arrange session index: %v", err)
	}

	sessions, total, err := repository.ListByUser(ctx, userID, 0, 2)
	if err != nil {
		t.Fatalf("list sessions: %v", err)
	}
	if total != 2 {
		t.Fatalf("total = %d, want 2 valid sessions", total)
	}
	if len(sessions) != 2 {
		t.Fatalf("page length = %d, want 2; sessions = %#v", len(sessions), sessions)
	}
	if sessions[0].ID != newest.ID || sessions[1].ID != oldest.ID {
		t.Fatalf("session order = [%s, %s], want [%s, %s]", sessions[0].ID, sessions[1].ID, newest.ID, oldest.ID)
	}
}

func TestDeleteByUserIsAtomicInRedis(t *testing.T) {
	address := os.Getenv("GOAT_REDIS_ADDR")
	if address == "" {
		t.Skip("set GOAT_REDIS_ADDR to run Redis integration tests")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	client := redis.NewClient(&redis.Options{Addr: address})
	t.Cleanup(func() { _ = client.Close() })
	if err := client.Ping(ctx).Err(); err != nil {
		t.Fatalf("ping Redis: %v", err)
	}

	now := time.Now().UTC()
	owned := &entity.Session{
		ID:                pkg.ULIDGenerator(),
		UserID:            "integration-user-" + pkg.ULIDGenerator(),
		CredentialVersion: entity.InitialCredentialVersion,
		CurrentJTI:        pkg.ULIDGenerator(),
		CreatedAt:         now,
		UpdatedAt:         now,
		ExpiresAt:         now.Add(time.Hour),
		SessionTTLSeconds: int64(time.Hour.Seconds()),
		JTITTLSeconds:     int64(time.Hour.Seconds()),
	}
	foreign := &entity.Session{
		ID:                pkg.ULIDGenerator(),
		UserID:            "integration-user-" + pkg.ULIDGenerator(),
		CredentialVersion: entity.InitialCredentialVersion,
		CurrentJTI:        pkg.ULIDGenerator(),
		CreatedAt:         now,
		UpdatedAt:         now,
		ExpiresAt:         now.Add(time.Hour),
		SessionTTLSeconds: int64(time.Hour.Seconds()),
		JTITTLSeconds:     int64(time.Hour.Seconds()),
	}

	t.Cleanup(func() {
		_ = client.Del(
			context.Background(),
			pkg.RedisKey("session", "id", owned.ID),
			pkg.RedisKey("session", "jti", owned.CurrentJTI),
			pkg.RedisKey("session", "user", owned.UserID),
			pkg.RedisKey("session", "id", foreign.ID),
			pkg.RedisKey("session", "jti", foreign.CurrentJTI),
			pkg.RedisKey("session", "user", foreign.UserID),
		).Err()
	})

	repository := &Repository{client: client}
	if err := repository.Create(ctx, owned); err != nil {
		t.Fatalf("create owned session: %v", err)
	}
	if err := repository.Create(ctx, foreign); err != nil {
		t.Fatalf("create foreign session: %v", err)
	}

	deleted, err := repository.DeleteByUser(ctx, owned.UserID, []string{owned.ID, foreign.ID})
	if err != nil {
		t.Fatalf("mixed-ownership delete: %v", err)
	}
	if deleted {
		t.Fatal("mixed-ownership delete reported success")
	}
	assertRedisSessionPresent(t, ctx, client, owned)
	assertRedisSessionPresent(t, ctx, client, foreign)

	deleted, err = repository.DeleteByUser(ctx, owned.UserID, []string{owned.ID})
	if err != nil {
		t.Fatalf("owned delete: %v", err)
	}
	if !deleted {
		t.Fatal("owned delete reported not found")
	}
	assertRedisSessionAbsent(t, ctx, client, owned)
	assertRedisSessionPresent(t, ctx, client, foreign)

	deleted, err = repository.DeleteByUser(ctx, owned.UserID, []string{pkg.ULIDGenerator()})
	if err != nil {
		t.Fatalf("missing-session delete: %v", err)
	}
	if deleted {
		t.Fatal("missing-session delete reported success")
	}
}

func TestDeleteOthersByUserIsAtomicInRedis(t *testing.T) {
	address := os.Getenv("GOAT_REDIS_ADDR")
	if address == "" {
		t.Skip("set GOAT_REDIS_ADDR to run Redis integration tests")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	client := redis.NewClient(&redis.Options{Addr: address})
	t.Cleanup(func() { _ = client.Close() })
	if err := client.Ping(ctx).Err(); err != nil {
		t.Fatalf("ping Redis: %v", err)
	}
	repository := &Repository{client: client}

	t.Run("keeps current and removes multiple other sessions", func(t *testing.T) {
		userID := "integration-user-" + pkg.ULIDGenerator()
		current := newIntegrationSession(userID, time.Hour)
		otherOne := newIntegrationSession(userID, time.Hour)
		otherTwo := newIntegrationSession(userID, time.Hour)
		registerRedisSessionCleanup(t, client, current, otherOne, otherTwo)
		createRedisSessions(t, ctx, repository, current, otherOne, otherTwo)

		currentOwned, err := repository.DeleteOthersByUser(ctx, userID, current.ID)
		if err != nil {
			t.Fatalf("delete other sessions: %v", err)
		}
		if !currentOwned {
			t.Fatal("current session reported missing or foreign")
		}
		assertRedisSessionPresent(t, ctx, client, current)
		assertRedisSessionAbsent(t, ctx, client, otherOne)
		assertRedisSessionAbsent(t, ctx, client, otherTwo)
	})

	t.Run("no other sessions and repeated execution are successful", func(t *testing.T) {
		current := newIntegrationSession("integration-user-"+pkg.ULIDGenerator(), time.Hour)
		registerRedisSessionCleanup(t, client, current)
		createRedisSessions(t, ctx, repository, current)

		for attempt := 0; attempt < 2; attempt++ {
			currentOwned, err := repository.DeleteOthersByUser(ctx, current.UserID, current.ID)
			if err != nil {
				t.Fatalf("delete other sessions attempt %d: %v", attempt+1, err)
			}
			if !currentOwned {
				t.Fatalf("attempt %d reported current session missing or foreign", attempt+1)
			}
		}
		assertRedisSessionPresent(t, ctx, client, current)
	})

	t.Run("stale index members are removed", func(t *testing.T) {
		current := newIntegrationSession("integration-user-"+pkg.ULIDGenerator(), time.Hour)
		registerRedisSessionCleanup(t, client, current)
		createRedisSessions(t, ctx, repository, current)
		staleSessionKey := pkg.RedisKey("session", "id", pkg.ULIDGenerator())
		userKey := pkg.RedisKey("session", "user", current.UserID)
		if err := client.ZAdd(ctx, userKey, redis.Z{Score: float64(time.Now().Unix()), Member: staleSessionKey}).Err(); err != nil {
			t.Fatalf("add stale index member: %v", err)
		}

		currentOwned, err := repository.DeleteOthersByUser(ctx, current.UserID, current.ID)
		if err != nil {
			t.Fatalf("delete other sessions: %v", err)
		}
		if !currentOwned {
			t.Fatal("current session reported missing or foreign")
		}
		assertRedisSessionPresent(t, ctx, client, current)
		assertRedisIndexMemberAbsent(t, ctx, client, userKey, staleSessionKey)
	})

	t.Run("expired session hashes leave no stale index member", func(t *testing.T) {
		userID := "integration-user-" + pkg.ULIDGenerator()
		expiring := newIntegrationSession(userID, time.Second)
		current := newIntegrationSession(userID, time.Hour)
		registerRedisSessionCleanup(t, client, expiring, current)

		// Creating the current session last extends the shared user-index TTL while
		// the older session hash and JTI expire naturally.
		createRedisSessions(t, ctx, repository, expiring, current)
		waitForRedisKeysToExpire(t, ctx, client,
			pkg.RedisKey("session", "id", expiring.ID),
			pkg.RedisKey("session", "jti", expiring.CurrentJTI),
		)
		userKey := pkg.RedisKey("session", "user", userID)
		if err := client.ZScore(ctx, userKey, pkg.RedisKey("session", "id", expiring.ID)).Err(); err != nil {
			t.Fatalf("expired session was not retained as a stale index member: %v", err)
		}

		currentOwned, err := repository.DeleteOthersByUser(ctx, userID, current.ID)
		if err != nil {
			t.Fatalf("delete other sessions: %v", err)
		}
		if !currentOwned {
			t.Fatal("current session reported missing or foreign")
		}
		assertRedisSessionPresent(t, ctx, client, current)
		assertRedisIndexMemberAbsent(t, ctx, client, userKey, pkg.RedisKey("session", "id", expiring.ID))
	})

	t.Run("foreign sessions are not deleted", func(t *testing.T) {
		userID := "integration-user-" + pkg.ULIDGenerator()
		current := newIntegrationSession(userID, time.Hour)
		ownedOther := newIntegrationSession(userID, time.Hour)
		foreign := newIntegrationSession("integration-user-"+pkg.ULIDGenerator(), time.Hour)
		registerRedisSessionCleanup(t, client, current, ownedOther, foreign)
		createRedisSessions(t, ctx, repository, current, ownedOther, foreign)

		userKey := pkg.RedisKey("session", "user", userID)
		foreignSessionKey := pkg.RedisKey("session", "id", foreign.ID)
		if err := client.ZAdd(ctx, userKey, redis.Z{Score: float64(time.Now().Unix()), Member: foreignSessionKey}).Err(); err != nil {
			t.Fatalf("add foreign index member: %v", err)
		}

		currentOwned, err := repository.DeleteOthersByUser(ctx, userID, current.ID)
		if err != nil {
			t.Fatalf("delete other sessions: %v", err)
		}
		if !currentOwned {
			t.Fatal("current session reported missing or foreign")
		}
		assertRedisSessionPresent(t, ctx, client, current)
		assertRedisSessionAbsent(t, ctx, client, ownedOther)
		assertRedisSessionPresent(t, ctx, client, foreign)
		assertRedisIndexMemberAbsent(t, ctx, client, userKey, foreignSessionKey)
	})

	t.Run("invalid current ownership causes zero mutation", func(t *testing.T) {
		userID := "integration-user-" + pkg.ULIDGenerator()
		ownedOther := newIntegrationSession(userID, time.Hour)
		foreignCurrent := newIntegrationSession("integration-user-"+pkg.ULIDGenerator(), time.Hour)
		registerRedisSessionCleanup(t, client, ownedOther, foreignCurrent)
		createRedisSessions(t, ctx, repository, ownedOther, foreignCurrent)

		userKey := pkg.RedisKey("session", "user", userID)
		staleSessionKey := pkg.RedisKey("session", "id", pkg.ULIDGenerator())
		if err := client.ZAdd(ctx, userKey, redis.Z{Score: float64(time.Now().Unix()), Member: staleSessionKey}).Err(); err != nil {
			t.Fatalf("add stale index member: %v", err)
		}

		currentOwned, err := repository.DeleteOthersByUser(ctx, userID, foreignCurrent.ID)
		if err != nil {
			t.Fatalf("delete other sessions: %v", err)
		}
		if currentOwned {
			t.Fatal("foreign current session reported as owned")
		}
		assertRedisSessionPresent(t, ctx, client, ownedOther)
		assertRedisSessionPresent(t, ctx, client, foreignCurrent)
		if err := client.ZScore(ctx, userKey, staleSessionKey).Err(); err != nil {
			t.Fatalf("stale index member was mutated after failed validation: %v", err)
		}
	})

	t.Run("concurrent creation is fully before or after atomic deletion", func(t *testing.T) {
		userID := "integration-user-" + pkg.ULIDGenerator()
		current := newIntegrationSession(userID, time.Hour)
		concurrent := newIntegrationSession(userID, time.Hour)
		registerRedisSessionCleanup(t, client, current, concurrent)
		createRedisSessions(t, ctx, repository, current)

		start := make(chan struct{})
		var wait sync.WaitGroup
		wait.Add(2)
		var createErr error
		var deleteErr error
		var currentOwned bool
		go func() {
			defer wait.Done()
			<-start
			createErr = repository.Create(ctx, concurrent)
		}()
		go func() {
			defer wait.Done()
			<-start
			currentOwned, deleteErr = repository.DeleteOthersByUser(ctx, userID, current.ID)
		}()
		close(start)
		wait.Wait()

		if createErr != nil {
			t.Fatalf("create concurrent session: %v", createErr)
		}
		if deleteErr != nil {
			t.Fatalf("delete other sessions: %v", deleteErr)
		}
		if !currentOwned {
			t.Fatal("current session reported missing or foreign")
		}
		assertRedisSessionPresent(t, ctx, client, current)
		assertRedisSessionStateIsComplete(t, ctx, client, concurrent)
	})
}

func newIntegrationSession(userID string, ttl time.Duration) *entity.Session {
	now := time.Now().UTC()
	return &entity.Session{
		ID:                pkg.ULIDGenerator(),
		UserID:            userID,
		CredentialVersion: entity.InitialCredentialVersion,
		CurrentJTI:        pkg.ULIDGenerator(),
		CreatedAt:         now,
		UpdatedAt:         now,
		ExpiresAt:         now.Add(ttl),
		SessionTTLSeconds: int64(ttl.Seconds()),
		JTITTLSeconds:     int64(ttl.Seconds()),
	}
}

func createRedisSessions(t *testing.T, ctx context.Context, repository *Repository, sessions ...*entity.Session) {
	t.Helper()
	for _, current := range sessions {
		if err := repository.Create(ctx, current); err != nil {
			t.Fatalf("create session %q: %v", current.ID, err)
		}
	}
}

func registerRedisSessionCleanup(t *testing.T, client *redis.Client, sessions ...*entity.Session) {
	t.Helper()
	keys := make([]string, 0, len(sessions)*3)
	seen := make(map[string]struct{}, len(sessions)*3)
	for _, current := range sessions {
		for _, key := range []string{
			pkg.RedisKey("session", "id", current.ID),
			pkg.RedisKey("session", "jti", current.CurrentJTI),
			pkg.RedisKey("session", "user", current.UserID),
		} {
			if _, exists := seen[key]; exists {
				continue
			}
			seen[key] = struct{}{}
			keys = append(keys, key)
		}
	}
	t.Cleanup(func() {
		_ = client.Del(context.Background(), keys...).Err()
	})
}

func waitForRedisKeysToExpire(t *testing.T, ctx context.Context, client *redis.Client, keys ...string) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		existing, err := client.Exists(ctx, keys...).Result()
		if err != nil {
			t.Fatalf("check expiring Redis keys: %v", err)
		}
		if existing == 0 {
			return
		}
		time.Sleep(25 * time.Millisecond)
	}
	t.Fatalf("Redis keys did not expire within deadline: %v", keys)
}

func assertRedisIndexMemberAbsent(t *testing.T, ctx context.Context, client *redis.Client, userKey, sessionKey string) {
	t.Helper()
	if err := client.ZScore(ctx, userKey, sessionKey).Err(); err != redis.Nil {
		t.Fatalf("session key %q still present in index %q or lookup failed: %v", sessionKey, userKey, err)
	}
}

func assertRedisSessionStateIsComplete(t *testing.T, ctx context.Context, client *redis.Client, session *entity.Session) {
	t.Helper()
	hashExists, err := client.Exists(ctx, pkg.RedisKey("session", "id", session.ID)).Result()
	if err != nil {
		t.Fatalf("check session hash: %v", err)
	}
	jtiExists, err := client.Exists(ctx, pkg.RedisKey("session", "jti", session.CurrentJTI)).Result()
	if err != nil {
		t.Fatalf("check session JTI: %v", err)
	}
	indexErr := client.ZScore(
		ctx,
		pkg.RedisKey("session", "user", session.UserID),
		pkg.RedisKey("session", "id", session.ID),
	).Err()
	indexed := indexErr == nil
	if indexErr != nil && indexErr != redis.Nil {
		t.Fatalf("check session index: %v", indexErr)
	}
	present := hashExists == 1
	if (jtiExists == 1) != present || indexed != present {
		t.Fatalf("concurrent session has partial state: hash=%d jti=%d indexed=%t", hashExists, jtiExists, indexed)
	}
}

func assertRedisSessionPresent(t *testing.T, ctx context.Context, client *redis.Client, session *entity.Session) {
	t.Helper()

	for _, key := range []string{
		pkg.RedisKey("session", "id", session.ID),
		pkg.RedisKey("session", "jti", session.CurrentJTI),
	} {
		exists, err := client.Exists(ctx, key).Result()
		if err != nil {
			t.Fatalf("check Redis key %q: %v", key, err)
		}
		if exists != 1 {
			t.Fatalf("Redis key %q does not exist", key)
		}
	}
	if err := client.ZScore(ctx, pkg.RedisKey("session", "user", session.UserID), pkg.RedisKey("session", "id", session.ID)).Err(); err != nil {
		t.Fatalf("session %q missing from user index: %v", session.ID, err)
	}
}

func assertRedisSessionAbsent(t *testing.T, ctx context.Context, client *redis.Client, session *entity.Session) {
	t.Helper()

	for _, key := range []string{
		pkg.RedisKey("session", "id", session.ID),
		pkg.RedisKey("session", "jti", session.CurrentJTI),
	} {
		exists, err := client.Exists(ctx, key).Result()
		if err != nil {
			t.Fatalf("check Redis key %q: %v", key, err)
		}
		if exists != 0 {
			t.Fatalf("Redis key %q still exists", key)
		}
	}
	err := client.ZScore(ctx, pkg.RedisKey("session", "user", session.UserID), pkg.RedisKey("session", "id", session.ID)).Err()
	if err != redis.Nil {
		t.Fatalf("session %q still present in user index or lookup failed: %v", session.ID, err)
	}
}
