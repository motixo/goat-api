package session

import (
	"context"
	"os"
	"sort"
	"sync"
	"testing"
	"time"

	"github.com/motixo/goat-api/internal/domain/entity"
	redisStorage "github.com/motixo/goat-api/internal/infra/storage/redis"
	"github.com/motixo/goat-api/internal/pkg"
	"github.com/redis/go-redis/v9"
)

func TestListByUserReturnsNoSessions(t *testing.T) {
	ctx, _, repository := newSessionListIntegrationRepository(t)

	sessions, total, err := repository.ListByUser(ctx, "integration-user-"+pkg.ULIDGenerator(), 0, 10)
	if err != nil {
		t.Fatalf("list sessions: %v", err)
	}
	assertSessionList(t, sessions, total, nil, 0)
}

func TestListByUserPaginatesInDeterministicOrder(t *testing.T) {
	ctx, client, repository := newSessionListIntegrationRepository(t)
	userID := "integration-user-" + pkg.ULIDGenerator()
	sessions := []*entity.Session{
		newIntegrationSession(userID, time.Hour),
		newIntegrationSession(userID, time.Hour),
		newIntegrationSession(userID, time.Hour),
		newIntegrationSession(userID, time.Hour),
		newIntegrationSession(userID, time.Hour),
	}
	registerRedisSessionCleanup(t, client, sessions...)
	createRedisSessions(t, ctx, repository, sessions...)

	userKey := pkg.RedisKey("session", "user", userID)
	for _, current := range sessions {
		if err := client.ZAdd(ctx, userKey, redis.Z{
			Score:  100,
			Member: pkg.RedisKey("session", "id", current.ID),
		}).Err(); err != nil {
			t.Fatalf("set deterministic session score: %v", err)
		}
	}

	ordered := append([]*entity.Session(nil), sessions...)
	sort.Slice(ordered, func(i, j int) bool { return ordered[i].ID > ordered[j].ID })
	wantAll := sessionIDs(ordered)

	tests := []struct {
		name   string
		offset int
		limit  int
		want   []string
	}{
		{name: "first page", offset: 0, limit: 2, want: wantAll[0:2]},
		{name: "second page", offset: 2, limit: 2, want: wantAll[2:4]},
		{name: "final partial page", offset: 4, limit: 2, want: wantAll[4:5]},
		{name: "past final page", offset: 5, limit: 2, want: nil},
		{name: "all sessions for internal callers", offset: 0, limit: 0, want: wantAll},
		{name: "maximum public page size", offset: 0, limit: 100, want: wantAll},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			for attempt := 1; attempt <= 2; attempt++ {
				got, total, err := repository.ListByUser(ctx, userID, test.offset, test.limit)
				if err != nil {
					t.Fatalf("list sessions attempt %d: %v", attempt, err)
				}
				assertSessionList(t, got, total, test.want, int64(len(wantAll)))
			}
		})
	}
}

func TestListByUserFiltersAndCleansInvalidIndexEntries(t *testing.T) {
	ctx, client, repository := newSessionListIntegrationRepository(t)
	userID := "integration-user-" + pkg.ULIDGenerator()
	newest := newIntegrationSession(userID, time.Hour)
	oldest := newIntegrationSession(userID, time.Hour)
	foreign := newIntegrationSession("integration-user-"+pkg.ULIDGenerator(), time.Hour)
	incomplete := newIntegrationSession(userID, time.Hour)
	poisoned := newIntegrationSession(userID, time.Hour)
	registerRedisSessionCleanup(t, client, newest, oldest, foreign, incomplete, poisoned)
	createRedisSessions(t, ctx, repository, newest, oldest, foreign)

	userKey := pkg.RedisKey("session", "user", userID)
	staleKey := pkg.RedisKey("session", "id", pkg.ULIDGenerator())
	wrongTypeKey := pkg.RedisKey("session", "id", pkg.ULIDGenerator())
	incompleteKey := pkg.RedisKey("session", "id", incomplete.ID)
	incompleteJTIKey := pkg.RedisKey("session", "jti", incomplete.CurrentJTI)
	poisonedKey := pkg.RedisKey("session", "id", poisoned.ID)
	foreignKey := pkg.RedisKey("session", "id", foreign.ID)
	t.Cleanup(func() {
		_ = client.Del(context.Background(), staleKey, wrongTypeKey).Err()
	})

	if err := client.Set(ctx, wrongTypeKey, "not-a-session-hash", time.Hour).Err(); err != nil {
		t.Fatalf("create wrong-type session key: %v", err)
	}
	if err := client.HSet(ctx, incompleteKey,
		"id", incomplete.ID,
		"user_id", incomplete.UserID,
		"current_jti", incomplete.CurrentJTI,
		"created_at", incomplete.CreatedAt.Unix(),
		"expires_at", incomplete.ExpiresAt.Unix(),
	).Err(); err != nil {
		t.Fatalf("create incomplete session hash: %v", err)
	}
	if err := client.Expire(ctx, incompleteKey, time.Hour).Err(); err != nil {
		t.Fatalf("expire incomplete session hash: %v", err)
	}
	if err := client.Set(ctx, incompleteJTIKey, incompleteKey, time.Hour).Err(); err != nil {
		t.Fatalf("create incomplete session JTI: %v", err)
	}
	if err := client.HSet(ctx, poisonedKey,
		"id", poisoned.ID,
		"user_id", poisoned.UserID,
		"current_jti", foreign.CurrentJTI,
		"created_at", poisoned.CreatedAt.Unix(),
		"expires_at", poisoned.ExpiresAt.Unix(),
	).Err(); err != nil {
		t.Fatalf("create poisoned session hash: %v", err)
	}
	if err := client.Expire(ctx, poisonedKey, time.Hour).Err(); err != nil {
		t.Fatalf("expire poisoned session hash: %v", err)
	}

	if err := client.ZAdd(ctx, userKey,
		redis.Z{Score: 70, Member: staleKey},
		redis.Z{Score: 60, Member: wrongTypeKey},
		redis.Z{Score: 50, Member: foreignKey},
		redis.Z{Score: 40, Member: incompleteKey},
		redis.Z{Score: 30, Member: poisonedKey},
		redis.Z{Score: 20, Member: pkg.RedisKey("session", "id", newest.ID)},
		redis.Z{Score: 10, Member: pkg.RedisKey("session", "id", oldest.ID)},
	).Err(); err != nil {
		t.Fatalf("arrange mixed session index: %v", err)
	}

	for attempt := 1; attempt <= 2; attempt++ {
		got, total, err := repository.ListByUser(ctx, userID, 0, 2)
		if err != nil {
			t.Fatalf("list sessions attempt %d: %v", attempt, err)
		}
		assertSessionList(t, got, total, []string{newest.ID, oldest.ID}, 2)
	}

	for _, invalidKey := range []string{staleKey, wrongTypeKey, foreignKey, incompleteKey, poisonedKey} {
		assertRedisIndexMemberAbsent(t, ctx, client, userKey, invalidKey)
	}
	assertRedisSessionPresent(t, ctx, client, foreign)
	assertRedisSessionAbsent(t, ctx, client, incomplete)
	if exists, err := client.Exists(ctx, poisonedKey).Result(); err != nil {
		t.Fatalf("check poisoned session hash: %v", err)
	} else if exists != 0 {
		t.Fatal("poisoned owned session hash was not revoked")
	}
	if exists, err := client.Exists(ctx, wrongTypeKey).Result(); err != nil {
		t.Fatalf("check wrong-type key: %v", err)
	} else if exists != 1 {
		t.Fatal("wrong-type key was changed without provable ownership")
	}
}

func TestListByUserRemovesExpiredSessionReferences(t *testing.T) {
	ctx, client, repository := newSessionListIntegrationRepository(t)
	userID := "integration-user-" + pkg.ULIDGenerator()
	expiring := newIntegrationSession(userID, time.Second)
	current := newIntegrationSession(userID, time.Hour)
	registerRedisSessionCleanup(t, client, expiring, current)

	// Creating the long-lived session last keeps the shared index alive after
	// the older hash and its JTI naturally expire.
	createRedisSessions(t, ctx, repository, expiring, current)
	waitForRedisKeysToExpire(t, ctx, client,
		pkg.RedisKey("session", "id", expiring.ID),
		pkg.RedisKey("session", "jti", expiring.CurrentJTI),
	)
	userKey := pkg.RedisKey("session", "user", userID)
	expiringKey := pkg.RedisKey("session", "id", expiring.ID)
	if err := client.ZScore(ctx, userKey, expiringKey).Err(); err != nil {
		t.Fatalf("expired session was not retained as a stale index reference: %v", err)
	}

	got, total, err := repository.ListByUser(ctx, userID, 0, 10)
	if err != nil {
		t.Fatalf("list sessions: %v", err)
	}
	assertSessionList(t, got, total, []string{current.ID}, 1)
	assertRedisIndexMemberAbsent(t, ctx, client, userKey, expiringKey)
}

func TestListByUserIndexLivesAsLongAsActiveSessions(t *testing.T) {
	t.Run("later short session does not shorten the shared index", func(t *testing.T) {
		ctx, client, repository := newSessionListIntegrationRepository(t)
		userID := "integration-user-" + pkg.ULIDGenerator()
		longLived := newIntegrationSession(userID, 5*time.Second)
		shortLived := newIntegrationSession(userID, time.Second)
		registerRedisSessionCleanup(t, client, longLived, shortLived)
		createRedisSessions(t, ctx, repository, longLived, shortLived)

		waitPastOriginalIndexTTL(t)
		assertRedisSessionPresent(t, ctx, client, longLived)
		got, total, err := repository.ListByUser(ctx, userID, 0, 10)
		if err != nil {
			t.Fatalf("list sessions: %v", err)
		}
		assertSessionList(t, got, total, []string{longLived.ID}, 1)
	})

	t.Run("rotation extends the shared index", func(t *testing.T) {
		ctx, client, repository := newSessionListIntegrationRepository(t)
		current := newIntegrationSession("integration-user-"+pkg.ULIDGenerator(), time.Second)
		registerRedisSessionCleanup(t, client, current)
		createRedisSessions(t, ctx, repository, current)

		newJTI := pkg.ULIDGenerator()
		newJTIKey := pkg.RedisKey("session", "jti", newJTI)
		t.Cleanup(func() { _ = client.Del(context.Background(), newJTIKey).Err() })
		rotatedID, err := repository.RotateJTI(
			ctx,
			current.CurrentJTI,
			newJTI,
			current.IP,
			current.Device,
			time.Now().UTC().Add(5*time.Second),
			5,
			5,
		)
		if err != nil {
			t.Fatalf("rotate session JTI: %v", err)
		}
		if rotatedID != current.ID {
			t.Fatalf("rotated session ID = %q, want %q", rotatedID, current.ID)
		}

		waitPastOriginalIndexTTL(t)
		if exists, err := client.Exists(ctx, pkg.RedisKey("session", "id", current.ID), newJTIKey).Result(); err != nil {
			t.Fatalf("check rotated session state: %v", err)
		} else if exists != 2 {
			t.Fatalf("rotated session keys present = %d, want 2", exists)
		}
		got, total, err := repository.ListByUser(ctx, current.UserID, 0, 10)
		if err != nil {
			t.Fatalf("list sessions: %v", err)
		}
		assertSessionList(t, got, total, []string{current.ID}, 1)
	})
}

func TestListByUserIsLinearizableWithConcurrentExpiry(t *testing.T) {
	ctx, client, repository := newSessionListIntegrationRepository(t)
	userID := "integration-user-" + pkg.ULIDGenerator()
	stable := newIntegrationSession(userID, time.Hour)
	expiring := newIntegrationSession(userID, time.Hour)
	registerRedisSessionCleanup(t, client, stable, expiring)
	createRedisSessions(t, ctx, repository, stable, expiring)

	start := make(chan struct{})
	var wait sync.WaitGroup
	wait.Add(2)
	var listed []*entity.Session
	var total int64
	var listErr error
	var expiryErr error
	go func() {
		defer wait.Done()
		<-start
		listed, total, listErr = repository.ListByUser(ctx, userID, 0, 10)
	}()
	go func() {
		defer wait.Done()
		<-start
		_, expiryErr = client.TxPipelined(ctx, func(pipe redis.Pipeliner) error {
			pipe.PExpire(ctx, pkg.RedisKey("session", "id", expiring.ID), 0)
			pipe.PExpire(ctx, pkg.RedisKey("session", "jti", expiring.CurrentJTI), 0)
			return nil
		})
	}()
	close(start)
	wait.Wait()

	if listErr != nil {
		t.Fatalf("list sessions concurrently with expiry: %v", listErr)
	}
	if expiryErr != nil {
		t.Fatalf("expire session state: %v", expiryErr)
	}
	if total != 1 && total != 2 {
		t.Fatalf("concurrent total = %d, want a linearizable result of 1 or 2", total)
	}
	if int64(len(listed)) != total {
		t.Fatalf("concurrent page length = %d, total = %d", len(listed), total)
	}
	assertUniqueOwnedSessionList(t, listed, userID)

	got, finalTotal, err := repository.ListByUser(ctx, userID, 0, 10)
	if err != nil {
		t.Fatalf("list sessions after expiry: %v", err)
	}
	assertSessionList(t, got, finalTotal, []string{stable.ID}, 1)
}

func TestListByUserUsesOneRedisCommandAfterScriptLoad(t *testing.T) {
	ctx, setupClient, setupRepository := newSessionListIntegrationRepository(t)
	userID := "integration-user-" + pkg.ULIDGenerator()
	sessions := []*entity.Session{
		newIntegrationSession(userID, time.Hour),
		newIntegrationSession(userID, time.Hour),
		newIntegrationSession(userID, time.Hour),
	}
	registerRedisSessionCleanup(t, setupClient, sessions...)
	createRedisSessions(t, ctx, setupRepository, sessions...)

	counter := &redisCommandCounter{}
	client := redis.NewClient(&redis.Options{Addr: os.Getenv("GOAT_REDIS_ADDR")})
	client.AddHook(counter)
	t.Cleanup(func() { _ = client.Close() })
	if err := client.Ping(ctx).Err(); err != nil {
		t.Fatalf("ping counting Redis client: %v", err)
	}
	if err := redisStorage.GetScript("list_sessions").Load(ctx, client).Err(); err != nil {
		t.Fatalf("load list script: %v", err)
	}
	counter.Reset()

	repository := &Repository{client: client}
	got, total, err := repository.ListByUser(ctx, userID, 0, 100)
	if err != nil {
		t.Fatalf("list sessions: %v", err)
	}
	if len(got) != len(sessions) || total != int64(len(sessions)) {
		t.Fatalf("list result length = %d, total = %d; want %d", len(got), total, len(sessions))
	}
	commands := counter.Commands()
	if len(commands) != 1 || commands[0] != "evalsha" {
		t.Fatalf("Redis commands = %v, want exactly one evalsha", commands)
	}
}

func newSessionListIntegrationRepository(t *testing.T) (context.Context, *redis.Client, *Repository) {
	t.Helper()
	address := os.Getenv("GOAT_REDIS_ADDR")
	if address == "" {
		t.Skip("set GOAT_REDIS_ADDR to run Redis integration tests")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	t.Cleanup(cancel)
	client := redis.NewClient(&redis.Options{Addr: address})
	t.Cleanup(func() { _ = client.Close() })
	if err := client.Ping(ctx).Err(); err != nil {
		t.Fatalf("ping Redis: %v", err)
	}
	return ctx, client, &Repository{client: client}
}

func waitPastOriginalIndexTTL(t *testing.T) {
	t.Helper()
	timer := time.NewTimer(1500 * time.Millisecond)
	defer timer.Stop()
	<-timer.C
}

func sessionIDs(sessions []*entity.Session) []string {
	ids := make([]string, 0, len(sessions))
	for _, current := range sessions {
		ids = append(ids, current.ID)
	}
	return ids
}

func assertSessionList(t *testing.T, sessions []*entity.Session, total int64, wantIDs []string, wantTotal int64) {
	t.Helper()
	if total != wantTotal {
		t.Fatalf("session total = %d, want %d", total, wantTotal)
	}
	gotIDs := sessionIDs(sessions)
	if len(gotIDs) != len(wantIDs) {
		t.Fatalf("session IDs = %v, want %v; total = %d", gotIDs, wantIDs, total)
	}
	for i := range wantIDs {
		if gotIDs[i] != wantIDs[i] {
			t.Fatalf("session IDs = %v, want %v; total = %d", gotIDs, wantIDs, total)
		}
	}
}

func assertUniqueOwnedSessionList(t *testing.T, sessions []*entity.Session, userID string) {
	t.Helper()
	seen := make(map[string]struct{}, len(sessions))
	for _, current := range sessions {
		if current.UserID != userID {
			t.Fatalf("listed foreign session %q owned by %q", current.ID, current.UserID)
		}
		if _, exists := seen[current.ID]; exists {
			t.Fatalf("listed duplicate session %q", current.ID)
		}
		seen[current.ID] = struct{}{}
	}
}

type redisCommandCounter struct {
	mu       sync.Mutex
	commands []string
}

func (c *redisCommandCounter) DialHook(next redis.DialHook) redis.DialHook {
	return next
}

func (c *redisCommandCounter) ProcessHook(next redis.ProcessHook) redis.ProcessHook {
	return func(ctx context.Context, cmd redis.Cmder) error {
		c.record(cmd.Name())
		return next(ctx, cmd)
	}
}

func (c *redisCommandCounter) ProcessPipelineHook(next redis.ProcessPipelineHook) redis.ProcessPipelineHook {
	return func(ctx context.Context, commands []redis.Cmder) error {
		for _, command := range commands {
			c.record(command.Name())
		}
		return next(ctx, commands)
	}
}

func (c *redisCommandCounter) Reset() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.commands = nil
}

func (c *redisCommandCounter) Commands() []string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]string(nil), c.commands...)
}

func (c *redisCommandCounter) record(command string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.commands = append(c.commands, command)
}
