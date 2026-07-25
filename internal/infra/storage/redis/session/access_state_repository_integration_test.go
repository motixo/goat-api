package session

import (
	"context"
	"errors"
	"os"
	"sync"
	"testing"
	"time"

	domainErrors "github.com/motixo/goat-api/internal/domain/errors"
	redisStorage "github.com/motixo/goat-api/internal/infra/storage/redis"
	"github.com/motixo/goat-api/internal/pkg"
	"github.com/redis/go-redis/v9"
)

func TestUserAccessStateBlockDeleteAndReactivate(t *testing.T) {
	ctx, client, repository := newSessionListIntegrationRepository(t)
	userID := "access-state-user-" + pkg.ULIDGenerator()
	current := newIntegrationSession(userID, time.Hour)
	other := newIntegrationSession(userID, time.Hour)
	foreign := newIntegrationSession("foreign-"+pkg.ULIDGenerator(), time.Hour)
	registerRedisSessionCleanup(t, client, current, other, foreign)
	createRedisSessions(t, ctx, repository, current, other, foreign)

	userKey := pkg.RedisKey("session", "user", userID)
	foreignKey := pkg.RedisKey("session", "id", foreign.ID)
	staleKey := pkg.RedisKey("session", "id", "stale-"+pkg.ULIDGenerator())
	wrongTypeKey := pkg.RedisKey("session", "id", "wrong-type-"+pkg.ULIDGenerator())
	incompleteKey := pkg.RedisKey("session", "id", "incomplete-"+pkg.ULIDGenerator())
	incompleteJTI := pkg.ULIDGenerator()
	incompleteJTIKey := pkg.RedisKey("session", "jti", incompleteJTI)
	t.Cleanup(func() {
		_ = client.Del(
			context.Background(),
			staleKey,
			wrongTypeKey,
			incompleteKey,
			incompleteJTIKey,
		).Err()
	})
	if err := client.ZAdd(
		ctx,
		userKey,
		redis.Z{Score: 1, Member: foreignKey},
		redis.Z{Score: 2, Member: staleKey},
		redis.Z{Score: 3, Member: wrongTypeKey},
		redis.Z{Score: 4, Member: incompleteKey},
	).Err(); err != nil {
		t.Fatalf("seed mixed user index: %v", err)
	}
	if err := client.Set(ctx, wrongTypeKey, "not-a-session", time.Hour).Err(); err != nil {
		t.Fatalf("seed wrong-type session: %v", err)
	}
	if err := client.HSet(
		ctx,
		incompleteKey,
		"user_id", userID,
		"current_jti", incompleteJTI,
	).Err(); err != nil {
		t.Fatalf("seed incomplete session: %v", err)
	}
	if err := client.Set(ctx, incompleteJTIKey, incompleteKey, time.Hour).Err(); err != nil {
		t.Fatalf("seed incomplete JTI: %v", err)
	}

	before := readUserAccessState(t, ctx, client, userID)
	if before.blocked || before.generation != 1 {
		t.Fatalf("initial access state = %#v, want unblocked generation 1", before)
	}

	if err := repository.BlockAndDeleteAllByUser(ctx, userID); err != nil {
		t.Fatalf("BlockAndDeleteAllByUser() error = %v", err)
	}
	blocked := readUserAccessState(t, ctx, client, userID)
	if !blocked.blocked || blocked.generation != before.generation+1 {
		t.Fatalf("blocked access state = %#v, want blocked next generation", blocked)
	}
	assertRedisSessionAbsent(t, ctx, client, current)
	assertRedisSessionAbsent(t, ctx, client, other)
	if exists, err := client.Exists(ctx, userKey, incompleteKey, incompleteJTIKey).Result(); err != nil {
		t.Fatalf("check blocked owned state: %v", err)
	} else if exists != 0 {
		t.Fatalf("blocked owned/index keys remaining = %d, want 0", exists)
	}
	assertRedisSessionPresent(t, ctx, client, foreign)
	if value, err := client.Get(ctx, wrongTypeKey).Result(); err != nil || value != "not-a-session" {
		t.Fatalf("foreign/wrong-type key changed = (%q, %v)", value, err)
	}

	if _, err := repository.FindByJTI(
		ctx,
		current.CurrentJTI,
		current.UserID,
		current.ID,
		current.CredentialVersion,
	); !errors.Is(err, domainErrors.ErrUserAccessBlocked) {
		t.Fatalf("blocked FindByJTI() error = %v, want ErrUserAccessBlocked", err)
	}

	late := newIntegrationSession(userID, time.Hour)
	registerRedisSessionCleanup(t, client, late)
	if err := repository.Create(ctx, late); !errors.Is(err, domainErrors.ErrUserAccessBlocked) {
		t.Fatalf("blocked Create() error = %v, want ErrUserAccessBlocked", err)
	}
	assertRedisSessionAbsent(t, ctx, client, late)

	if err := repository.BlockAndDeleteAllByUser(ctx, userID); err != nil {
		t.Fatalf("repeated BlockAndDeleteAllByUser() error = %v", err)
	}
	repeated := readUserAccessState(t, ctx, client, userID)
	if repeated != blocked {
		t.Fatalf("repeated block state = %#v, want idempotent %#v", repeated, blocked)
	}

	if err := repository.UnblockUser(ctx, userID); err != nil {
		t.Fatalf("UnblockUser() error = %v", err)
	}
	active := readUserAccessState(t, ctx, client, userID)
	if active.blocked || active.generation != blocked.generation {
		t.Fatalf("reactivated access state = %#v, want same generation unblocked", active)
	}
	if err := repository.UnblockUser(ctx, userID); err != nil {
		t.Fatalf("repeated UnblockUser() error = %v", err)
	}
	if repeatedActive := readUserAccessState(t, ctx, client, userID); repeatedActive != active {
		t.Fatalf("repeated unblock state = %#v, want %#v", repeatedActive, active)
	}

	relogin := newIntegrationSession(userID, time.Hour)
	registerRedisSessionCleanup(t, client, relogin)
	if err := repository.Create(ctx, relogin); err != nil {
		t.Fatalf("create session after reactivation: %v", err)
	}
	if relogin.SessionGeneration != active.generation {
		t.Fatalf(
			"new session generation = %d, want %d",
			relogin.SessionGeneration,
			active.generation,
		)
	}
}

func TestUnblockMissingAccessStateFailsClosedWithoutInitializingGeneration(
	t *testing.T,
) {
	ctx, client, repository := newSessionListIntegrationRepository(t)
	userID := "inactive-activation-" + pkg.ULIDGenerator()
	accessKey := pkg.RedisKey("session", "access", userID)
	t.Cleanup(func() {
		_ = client.Del(context.Background(), accessKey).Err()
	})

	if err := repository.UnblockUser(ctx, userID); err == nil {
		t.Fatal("UnblockUser(missing) error = nil, want fail-closed error")
	}
	if exists, err := client.Exists(ctx, accessKey).Result(); err != nil {
		t.Fatalf("check missing access state: %v", err)
	} else if exists != 0 {
		t.Fatalf("access state exists after missing unblock = %d, want 0", exists)
	}
}

func TestUserAccessStateMissingAndMalformedFailClosedWithoutPartialMutation(
	t *testing.T,
) {
	ctx, client, repository := newSessionListIntegrationRepository(t)
	current := newIntegrationSession("access-corruption-"+pkg.ULIDGenerator(), time.Hour)
	registerRedisSessionCleanup(t, client, current)
	createRedisSessions(t, ctx, repository, current)

	accessKey := pkg.RedisKey("session", "access", current.UserID)
	if err := client.Del(ctx, accessKey).Err(); err != nil {
		t.Fatalf("delete access state: %v", err)
	}
	found, err := repository.FindByJTI(
		ctx,
		current.CurrentJTI,
		current.UserID,
		current.ID,
		current.CredentialVersion,
	)
	if err != nil || found != nil {
		t.Fatalf("missing-state FindByJTI() = (%#v, %v), want (nil, nil)", found, err)
	}

	if err := client.HSet(
		ctx,
		accessKey,
		"blocked", "invalid",
		"session_generation", "1",
	).Err(); err != nil {
		t.Fatalf("seed malformed access state: %v", err)
	}
	if _, err := repository.FindByJTI(
		ctx,
		current.CurrentJTI,
		current.UserID,
		current.ID,
		current.CredentialVersion,
	); err == nil {
		t.Fatal("malformed-state FindByJTI() error = nil")
	}

	late := newIntegrationSession(current.UserID, time.Hour)
	registerRedisSessionCleanup(t, client, late)
	if err := repository.Create(ctx, late); err == nil {
		t.Fatal("malformed-state Create() error = nil")
	}
	assertRedisSessionAbsent(t, ctx, client, late)

	newJTI := pkg.ULIDGenerator()
	newJTIKey := pkg.RedisKey("session", "jti", newJTI)
	t.Cleanup(func() { _ = client.Del(context.Background(), newJTIKey).Err() })
	if _, err := repository.RotateJTI(
		ctx,
		current.CurrentJTI,
		newJTI,
		current.UserID,
		current.ID,
		current.CredentialVersion,
		current.IP,
		current.Device,
		time.Now().UTC().Add(time.Hour),
		int64(time.Hour.Seconds()),
		int64(time.Hour.Seconds()),
	); err == nil {
		t.Fatal("malformed-state RotateJTI() error = nil")
	}
	if exists, err := client.Exists(
		ctx,
		pkg.RedisKey("session", "jti", current.CurrentJTI),
		newJTIKey,
	).Result(); err != nil || exists != 1 {
		t.Fatalf("rotation mutated JTI keys = (%d, %v), want old only", exists, err)
	}
}

func TestBlockAccessStateRejectsMalformedStateWithoutPartialMutation(t *testing.T) {
	ctx, client, repository := newSessionListIntegrationRepository(t)
	current := newIntegrationSession("access-block-validation-"+pkg.ULIDGenerator(), time.Hour)
	registerRedisSessionCleanup(t, client, current)
	createRedisSessions(t, ctx, repository, current)

	accessKey := pkg.RedisKey("session", "access", current.UserID)
	if err := client.Del(ctx, accessKey).Err(); err != nil {
		t.Fatalf("delete valid access state: %v", err)
	}
	if err := client.Set(ctx, accessKey, "wrong-type", time.Hour).Err(); err != nil {
		t.Fatalf("seed wrong-type access state: %v", err)
	}

	if err := repository.BlockAndDeleteAllByUser(ctx, current.UserID); err == nil {
		t.Fatal("BlockAndDeleteAllByUser() error = nil for malformed access state")
	}
	assertRedisSessionPresent(t, ctx, client, current)
	if members, err := client.ZCard(
		ctx,
		pkg.RedisKey("session", "user", current.UserID),
	).Result(); err != nil || members != 1 {
		t.Fatalf("user index after failed block = (%d, %v), want one untouched member", members, err)
	}
	if value, err := client.Get(ctx, accessKey).Result(); err != nil || value != "wrong-type" {
		t.Fatalf("access state after failed block = (%q, %v), want original value", value, err)
	}
}

func TestBlockAccessStateWithNoSessionsIsIdempotentAndPrunesWrongTypeIndex(t *testing.T) {
	ctx, client, repository := newSessionListIntegrationRepository(t)
	userID := "access-no-sessions-" + pkg.ULIDGenerator()
	userKey := pkg.RedisKey("session", "user", userID)
	accessKey := pkg.RedisKey("session", "access", userID)
	t.Cleanup(func() {
		_ = client.Del(context.Background(), userKey, accessKey).Err()
	})
	if err := client.Set(ctx, userKey, "wrong-type", time.Hour).Err(); err != nil {
		t.Fatalf("seed wrong-type user index: %v", err)
	}

	if err := repository.BlockAndDeleteAllByUser(ctx, userID); err != nil {
		t.Fatalf("BlockAndDeleteAllByUser() error = %v", err)
	}
	first := readUserAccessState(t, ctx, client, userID)
	if !first.blocked || first.generation != 1 {
		t.Fatalf("first block state = %#v, want blocked generation 1", first)
	}
	if exists, err := client.Exists(ctx, userKey).Result(); err != nil || exists != 0 {
		t.Fatalf("wrong-type user index existence = (%d, %v), want absent", exists, err)
	}

	if err := repository.BlockAndDeleteAllByUser(ctx, userID); err != nil {
		t.Fatalf("repeated BlockAndDeleteAllByUser() error = %v", err)
	}
	if repeated := readUserAccessState(t, ctx, client, userID); repeated != first {
		t.Fatalf("repeated block state = %#v, want idempotent %#v", repeated, first)
	}
}

func TestBlockAccessStateUsesOneAtomicRedisOperationAfterScriptLoad(t *testing.T) {
	ctx, setupClient, setupRepository := newSessionListIntegrationRepository(t)
	current := newIntegrationSession("access-command-count-"+pkg.ULIDGenerator(), time.Hour)
	registerRedisSessionCleanup(t, setupClient, current)
	createRedisSessions(t, ctx, setupRepository, current)

	counter := &redisCommandCounter{}
	client := redis.NewClient(&redis.Options{Addr: os.Getenv("GOAT_REDIS_ADDR")})
	client.AddHook(counter)
	t.Cleanup(func() { _ = client.Close() })
	if err := client.Ping(ctx).Err(); err != nil {
		t.Fatalf("ping counting Redis client: %v", err)
	}
	script, err := redisStorage.GetScript(redisStorage.ScriptDeleteOtherSessions)
	if err != nil {
		t.Fatalf("resolve block-and-delete script: %v", err)
	}
	if err := script.Load(ctx, client).Err(); err != nil {
		t.Fatalf("load block-and-delete script: %v", err)
	}
	counter.Reset()

	repository := &Repository{client: client}
	if err := repository.BlockAndDeleteAllByUser(ctx, current.UserID); err != nil {
		t.Fatalf("BlockAndDeleteAllByUser() error = %v", err)
	}
	commands := counter.Commands()
	if len(commands) != 1 || commands[0] != "evalsha" {
		t.Fatalf("Redis commands = %v, want exactly one evalsha", commands)
	}
}

func TestConcurrentBlockLinearizesAgainstSessionCreationAndRotation(t *testing.T) {
	t.Run("validation", func(t *testing.T) {
		ctx, client, repository := newSessionListIntegrationRepository(t)
		current := newIntegrationSession("access-validate-race-"+pkg.ULIDGenerator(), time.Hour)
		registerRedisSessionCleanup(t, client, current)
		createRedisSessions(t, ctx, repository, current)

		start := make(chan struct{})
		var wait sync.WaitGroup
		wait.Add(2)
		var foundBeforeBlock bool
		var validationErr, blockErr error
		go func() {
			defer wait.Done()
			<-start
			found, err := repository.FindByJTI(
				ctx,
				current.CurrentJTI,
				current.UserID,
				current.ID,
				current.CredentialVersion,
			)
			validationErr = err
			foundBeforeBlock = found != nil
		}()
		go func() {
			defer wait.Done()
			<-start
			blockErr = repository.BlockAndDeleteAllByUser(ctx, current.UserID)
		}()
		close(start)
		wait.Wait()

		if blockErr != nil {
			t.Fatalf("block error = %v", blockErr)
		}
		if validationErr != nil &&
			!errors.Is(validationErr, domainErrors.ErrUserAccessBlocked) {
			t.Fatalf("validation error = %v, want nil or ErrUserAccessBlocked", validationErr)
		}
		t.Logf("validation linearized before access block: %t", foundBeforeBlock)
		if _, err := repository.FindByJTI(
			ctx,
			current.CurrentJTI,
			current.UserID,
			current.ID,
			current.CredentialVersion,
		); !errors.Is(err, domainErrors.ErrUserAccessBlocked) {
			t.Fatalf("post-block validation error = %v, want ErrUserAccessBlocked", err)
		}
	})

	t.Run("creation", func(t *testing.T) {
		ctx, client, repository := newSessionListIntegrationRepository(t)
		userID := "access-create-race-" + pkg.ULIDGenerator()
		seed := newIntegrationSession(userID, time.Hour)
		late := newIntegrationSession(userID, time.Hour)
		registerRedisSessionCleanup(t, client, seed, late)
		createRedisSessions(t, ctx, repository, seed)

		start := make(chan struct{})
		var wait sync.WaitGroup
		wait.Add(2)
		var createErr, blockErr error
		go func() {
			defer wait.Done()
			<-start
			createErr = repository.Create(ctx, late)
		}()
		go func() {
			defer wait.Done()
			<-start
			blockErr = repository.BlockAndDeleteAllByUser(ctx, userID)
		}()
		close(start)
		wait.Wait()

		if blockErr != nil {
			t.Fatalf("block error = %v", blockErr)
		}
		if createErr != nil && !errors.Is(createErr, domainErrors.ErrUserAccessBlocked) {
			t.Fatalf("create error = %v, want nil or ErrUserAccessBlocked", createErr)
		}
		assertRedisSessionAbsent(t, ctx, client, seed)
		assertRedisSessionAbsent(t, ctx, client, late)
		if state := readUserAccessState(t, ctx, client, userID); !state.blocked {
			t.Fatalf("final access state = %#v, want blocked", state)
		}
	})

	t.Run("rotation", func(t *testing.T) {
		ctx, client, repository := newSessionListIntegrationRepository(t)
		current := newIntegrationSession("access-rotate-race-"+pkg.ULIDGenerator(), time.Hour)
		registerRedisSessionCleanup(t, client, current)
		createRedisSessions(t, ctx, repository, current)
		newJTI := pkg.ULIDGenerator()
		newJTIKey := pkg.RedisKey("session", "jti", newJTI)
		t.Cleanup(func() { _ = client.Del(context.Background(), newJTIKey).Err() })

		start := make(chan struct{})
		var wait sync.WaitGroup
		wait.Add(2)
		var rotateErr, blockErr error
		go func() {
			defer wait.Done()
			<-start
			_, rotateErr = repository.RotateJTI(
				ctx,
				current.CurrentJTI,
				newJTI,
				current.UserID,
				current.ID,
				current.CredentialVersion,
				current.IP,
				current.Device,
				time.Now().UTC().Add(time.Hour),
				int64(time.Hour.Seconds()),
				int64(time.Hour.Seconds()),
			)
		}()
		go func() {
			defer wait.Done()
			<-start
			blockErr = repository.BlockAndDeleteAllByUser(ctx, current.UserID)
		}()
		close(start)
		wait.Wait()

		if blockErr != nil {
			t.Fatalf("block error = %v", blockErr)
		}
		if rotateErr != nil &&
			!errors.Is(rotateErr, domainErrors.ErrUserAccessBlocked) {
			t.Fatalf("rotate error = %v, want nil or ErrUserAccessBlocked", rotateErr)
		}
		assertRedisSessionAbsent(t, ctx, client, current)
		if exists, err := client.Exists(ctx, newJTIKey).Result(); err != nil || exists != 0 {
			t.Fatalf("rotated JTI after block = (%d, %v), want absent", exists, err)
		}
	})
}

type userAccessState struct {
	blocked    bool
	generation int64
}

func readUserAccessState(
	t *testing.T,
	ctx context.Context,
	client *redis.Client,
	userID string,
) userAccessState {
	t.Helper()
	fields, err := client.HMGet(
		ctx,
		pkg.RedisKey("session", "access", userID),
		"blocked",
		"session_generation",
	).Result()
	if err != nil {
		t.Fatalf("read user access state: %v", err)
	}
	if len(fields) != 2 {
		t.Fatalf("access-state field count = %d, want 2", len(fields))
	}
	blocked, ok := fields[0].(string)
	if !ok || (blocked != "0" && blocked != "1") {
		t.Fatalf("access-state blocked value = %#v", fields[0])
	}
	generation, err := client.HGet(
		ctx,
		pkg.RedisKey("session", "access", userID),
		"session_generation",
	).Int64()
	if err != nil || generation <= 0 {
		t.Fatalf("access-state generation = (%d, %v), want positive", generation, err)
	}
	return userAccessState{
		blocked:    blocked == "1",
		generation: generation,
	}
}
