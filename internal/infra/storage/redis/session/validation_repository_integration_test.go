package session

import (
	"os"
	"sync"
	"testing"
	"time"

	redisStorage "github.com/motixo/goat-api/internal/infra/storage/redis"
	"github.com/redis/go-redis/v9"
)

func TestFindByJTIUsesOneAtomicRedisOperationAfterScriptLoad(t *testing.T) {
	ctx, setupClient, setupRepository := newSessionListIntegrationRepository(t)
	current := newIntegrationSession("validation-user", time.Hour)
	registerRedisSessionCleanup(t, setupClient, current)
	createRedisSessions(t, ctx, setupRepository, current)

	counter := &redisCommandCounter{}
	client := redis.NewClient(&redis.Options{Addr: os.Getenv("GOAT_REDIS_ADDR")})
	client.AddHook(counter)
	t.Cleanup(func() { _ = client.Close() })
	if err := client.Ping(ctx).Err(); err != nil {
		t.Fatalf("ping counting Redis client: %v", err)
	}
	script, err := redisStorage.GetScript(redisStorage.ScriptGetSessionByJTI)
	if err != nil {
		t.Fatalf("resolve session-validation script: %v", err)
	}
	if err := script.Load(ctx, client).Err(); err != nil {
		t.Fatalf("load session-validation script: %v", err)
	}
	counter.Reset()

	repository := &Repository{client: client}
	got, err := repository.FindByJTI(
		ctx,
		current.CurrentJTI,
		current.UserID,
		current.ID,
		current.CredentialVersion,
	)
	if err != nil {
		t.Fatalf("FindByJTI() error = %v", err)
	}
	if got == nil ||
		got.ID != current.ID ||
		got.UserID != current.UserID ||
		got.CurrentJTI != current.CurrentJTI ||
		got.CredentialVersion != current.CredentialVersion {
		t.Fatalf("FindByJTI() = %#v, want validated session snapshot %#v", got, current)
	}
	commands := counter.Commands()
	if len(commands) != 1 || commands[0] != "evalsha" {
		t.Fatalf("Redis commands = %v, want exactly one evalsha", commands)
	}
}

func TestFindByJTIFailsClosedAndCleansIncompleteSessionReference(t *testing.T) {
	ctx, client, repository := newSessionListIntegrationRepository(t)
	current := newIntegrationSession("incomplete-validation-user", time.Hour)
	registerRedisSessionCleanup(t, client, current)
	createRedisSessions(t, ctx, repository, current)
	sessionKey := "session:id:" + current.ID
	jtiKey := "session:jti:" + current.CurrentJTI
	if err := client.HDel(ctx, sessionKey, "credential_version").Err(); err != nil {
		t.Fatalf("remove required session field: %v", err)
	}

	got, err := repository.FindByJTI(
		ctx,
		current.CurrentJTI,
		current.UserID,
		current.ID,
		current.CredentialVersion,
	)
	if err != nil {
		t.Fatalf("FindByJTI() error = %v", err)
	}
	if got != nil {
		t.Fatalf("FindByJTI() = %#v, want nil for incomplete session", got)
	}
	if exists, err := client.Exists(ctx, jtiKey).Result(); err != nil || exists != 0 {
		t.Fatalf("stale JTI key existence = (%d, %v), want (0, nil)", exists, err)
	}
}

func TestSessionValidationAndRevocationAreIndividuallyAtomic(t *testing.T) {
	ctx, client, repository := newSessionListIntegrationRepository(t)
	current := newIntegrationSession("validation-revocation-user", time.Hour)
	registerRedisSessionCleanup(t, client, current)
	createRedisSessions(t, ctx, repository, current)

	start := make(chan struct{})
	var wait sync.WaitGroup
	wait.Add(2)
	var validated bool
	var validationErr error
	var deleted bool
	var deletionErr error
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
		validated = found != nil
	}()
	go func() {
		defer wait.Done()
		<-start
		deleted, deletionErr = repository.DeleteByUser(
			ctx,
			current.UserID,
			[]string{current.ID},
		)
	}()
	close(start)
	wait.Wait()

	if validationErr != nil {
		t.Fatalf("concurrent FindByJTI() error = %v", validationErr)
	}
	if deletionErr != nil || !deleted {
		t.Fatalf("concurrent DeleteByUser() = (%t, %v), want (true, nil)", deleted, deletionErr)
	}
	// If validation linearized first, the already-authenticated in-flight
	// request may continue. Every validation that starts after revocation must
	// fail, which is the immediate revocation boundary Redis can guarantee.
	t.Logf("concurrent validation linearized before revocation: %t", validated)
	found, err := repository.FindByJTI(
		ctx,
		current.CurrentJTI,
		current.UserID,
		current.ID,
		current.CredentialVersion,
	)
	if err != nil {
		t.Fatalf("FindByJTI(after revocation) error = %v", err)
	}
	if found != nil {
		t.Fatalf("FindByJTI(after revocation) = %#v, want nil", found)
	}
}
