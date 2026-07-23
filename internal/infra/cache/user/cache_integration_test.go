package usercache

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/motixo/goat-api/internal/domain/entity"
	domainErrors "github.com/motixo/goat-api/internal/domain/errors"
	"github.com/motixo/goat-api/internal/domain/repository"
	"github.com/motixo/goat-api/internal/domain/valueobject"
	"github.com/motixo/goat-api/internal/pkg"
	"github.com/redis/go-redis/v9"
)

func TestUserAuthorizationCacheRoundTrip(t *testing.T) {
	ctx, client, cache := newUserCacheIntegration(t)
	userID := "11111111-1111-4111-8111-111111111111"
	registerUserCacheCleanup(t, client, userID)
	createdAt := time.Date(2026, time.July, 23, 16, 0, 0, 0, time.UTC)
	updatedAt := createdAt.Add(time.Hour)
	user := &entity.User{
		ID:        userID,
		Role:      valueobject.RoleAdmin,
		Status:    valueobject.StatusActive,
		CreatedAt: createdAt,
		UpdatedAt: &updatedAt,
	}

	if err := cache.Set(ctx, userID, user); err != nil {
		t.Fatalf("set cached user authorization: %v", err)
	}
	key := pkg.RedisKey("user", "id", userID)
	raw, err := client.Get(ctx, key).Result()
	if err != nil {
		t.Fatalf("get raw cached user authorization: %v", err)
	}
	const wantJSON = `{"id":"11111111-1111-4111-8111-111111111111","role":"admin","status":"active","created_at":"2026-07-23T16:00:00Z","updated_at":"2026-07-23T17:00:00Z"}`
	if raw != wantJSON {
		t.Fatalf("raw cached user authorization = %s, want %s", raw, wantJSON)
	}
	ttl, err := client.TTL(ctx, key).Result()
	if err != nil {
		t.Fatalf("get user cache TTL: %v", err)
	}
	if ttl <= 23*time.Hour || ttl > 24*time.Hour {
		t.Fatalf("user cache TTL = %s, want approximately 24h", ttl)
	}

	got, err := cache.Get(ctx, userID)
	if err != nil {
		t.Fatalf("get cached user authorization: %v", err)
	}
	assertUserAuthorization(t, got, user)
}

func TestUserAuthorizationCachePreservesAllRolesAndStatuses(t *testing.T) {
	ctx, client, cache := newUserCacheIntegration(t)
	statuses := []valueobject.UserStatus{
		valueobject.StatusInactive,
		valueobject.StatusActive,
		valueobject.StatusSuspended,
	}
	createdAt := time.Date(2026, time.July, 23, 16, 0, 0, 0, time.UTC)

	for _, role := range valueobject.AllRoles() {
		for _, status := range statuses {
			t.Run(role.String()+"/"+status.String(), func(t *testing.T) {
				userID := uuid.NewString()
				registerUserCacheCleanup(t, client, userID)
				user := &entity.User{ID: userID, Role: role, Status: status, CreatedAt: createdAt}
				if err := cache.Set(ctx, userID, user); err != nil {
					t.Fatalf("set cached user authorization: %v", err)
				}
				got, err := cache.Get(ctx, userID)
				if err != nil {
					t.Fatalf("get cached user authorization: %v", err)
				}
				assertUserAuthorization(t, got, user)
			})
		}
	}
}

func TestUserAuthorizationCacheMiss(t *testing.T) {
	ctx, client, cache := newUserCacheIntegration(t)
	userID := uuid.NewString()
	registerUserCacheCleanup(t, client, userID)

	got, err := cache.Get(ctx, userID)
	if err != nil {
		t.Fatalf("get missing cached user authorization: %v", err)
	}
	if got != nil {
		t.Fatalf("missing cached authorization = %#v, want nil", got)
	}
}

func TestUserAuthorizationCacheRejectsMalformedAndIncompletePayloads(t *testing.T) {
	ctx, client, cache := newUserCacheIntegration(t)
	const createdAt = `"2026-07-23T16:00:00Z"`
	tests := []struct {
		name    string
		payload func(userID string) string
	}{
		{name: "malformed JSON", payload: func(string) string { return `{"id":` }},
		{name: "null payload", payload: func(string) string { return `null` }},
		{name: "array payload", payload: func(string) string { return `[]` }},
		{
			name: "missing identity",
			payload: func(string) string {
				return fmt.Sprintf(`{"role":"client","status":"active","created_at":%s,"updated_at":null}`, createdAt)
			},
		},
		{
			name: "missing role",
			payload: func(userID string) string {
				return fmt.Sprintf(`{"id":%q,"status":"active","created_at":%s,"updated_at":null}`, userID, createdAt)
			},
		},
		{
			name: "missing status",
			payload: func(userID string) string {
				return fmt.Sprintf(`{"id":%q,"role":"client","created_at":%s,"updated_at":null}`, userID, createdAt)
			},
		},
		{
			name: "missing creation time",
			payload: func(userID string) string {
				return fmt.Sprintf(`{"id":%q,"role":"client","status":"active","updated_at":null}`, userID)
			},
		},
		{
			name: "malformed creation time",
			payload: func(userID string) string {
				return fmt.Sprintf(`{"id":%q,"role":"client","status":"active","created_at":"yesterday","updated_at":null}`, userID)
			},
		},
		{
			name: "unknown numeric role",
			payload: func(userID string) string {
				return fmt.Sprintf(`{"id":%q,"role":99,"status":"active","created_at":%s,"updated_at":null}`, userID, createdAt)
			},
		},
		{
			name: "unknown numeric status",
			payload: func(userID string) string {
				return fmt.Sprintf(`{"id":%q,"role":"client","status":99,"created_at":%s,"updated_at":null}`, userID, createdAt)
			},
		},
		{
			name: "unknown string role",
			payload: func(userID string) string {
				return fmt.Sprintf(`{"id":%q,"role":"owner","status":"active","created_at":%s,"updated_at":null}`, userID, createdAt)
			},
		},
		{
			name: "unknown string status",
			payload: func(userID string) string {
				return fmt.Sprintf(`{"id":%q,"role":"client","status":"deleted","created_at":%s,"updated_at":null}`, userID, createdAt)
			},
		},
		{
			name: "mismatched cache identity",
			payload: func(string) string {
				return fmt.Sprintf(`{"id":"22222222-2222-4222-8222-222222222222","role":"admin","status":"active","created_at":%s,"updated_at":null}`, createdAt)
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			userID := uuid.NewString()
			registerUserCacheCleanup(t, client, userID)
			key := pkg.RedisKey("user", "id", userID)
			if err := client.Set(ctx, key, test.payload(userID), time.Minute).Err(); err != nil {
				t.Fatalf("set invalid user cache payload: %v", err)
			}
			if _, err := cache.Get(ctx, userID); err == nil {
				t.Fatal("Get() error = nil, want invalid cache payload error")
			}
		})
	}
}

func TestCachedRepositoryDoesNotAuthorizeFromInvalidCachedData(t *testing.T) {
	ctx, client, cache := newUserCacheIntegration(t)
	createdAt := time.Date(2026, time.July, 23, 16, 0, 0, 0, time.UTC)
	tests := []struct {
		name    string
		payload func(userID string) string
	}{
		{
			name: "unknown numeric enums",
			payload: func(string) string {
				return `{"role":99,"status":99}`
			},
		},
		{
			name: "mismatched privileged identity",
			payload: func(string) string {
				return `{"id":"22222222-2222-4222-8222-222222222222","role":"admin","status":"active","created_at":"2026-07-23T16:00:00Z","updated_at":null}`
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			userID := uuid.NewString()
			registerUserCacheCleanup(t, client, userID)
			if err := client.Set(
				ctx,
				pkg.RedisKey("user", "id", userID),
				test.payload(userID),
				time.Minute,
			).Err(); err != nil {
				t.Fatalf("set poisoned user cache payload: %v", err)
			}

			database := &userRepositoryStub{user: &entity.User{
				ID: userID, Role: valueobject.RoleClient, Status: valueobject.StatusSuspended, CreatedAt: createdAt,
			}}
			logger := &recordingUserCacheLogger{}
			repository := NewCachedRepository(database, cache, logger)

			status, err := repository.GetUserStatus(ctx, userID)
			if err != nil {
				t.Fatalf("get authoritative user status: %v", err)
			}
			if status != valueobject.StatusSuspended {
				t.Fatalf("status = %d, want suspended", status)
			}
			role, err := repository.GetUserRole(ctx, userID)
			if err != nil {
				t.Fatalf("get authoritative user role: %v", err)
			}
			if role != valueobject.RoleClient {
				t.Fatalf("role = %d, want client", role)
			}
			if database.findByIDCalls != 1 {
				t.Fatalf("PostgreSQL fallback calls = %d, want 1", database.findByIDCalls)
			}
			if !logger.containsWarning("read user authorization cache failed") {
				t.Fatalf("warnings = %v, want invalid cache warning", logger.warnings)
			}
		})
	}
}

func TestCachedRepositoryFallsBackOnMissAndPropagatesPostgreSQLFailures(t *testing.T) {
	ctx, client, cache := newUserCacheIntegration(t)
	createdAt := time.Date(2026, time.July, 23, 16, 0, 0, 0, time.UTC)

	t.Run("cache miss uses and populates authoritative PostgreSQL result", func(t *testing.T) {
		userID := uuid.NewString()
		registerUserCacheCleanup(t, client, userID)
		database := &userRepositoryStub{user: &entity.User{
			ID: userID, Role: valueobject.RoleOperator, Status: valueobject.StatusActive, CreatedAt: createdAt,
		}}
		repository := NewCachedRepository(database, cache, &recordingUserCacheLogger{})

		role, err := repository.GetUserRole(ctx, userID)
		if err != nil {
			t.Fatalf("get user role: %v", err)
		}
		if role != valueobject.RoleOperator {
			t.Fatalf("role = %d, want operator", role)
		}
		if database.findByIDCalls != 1 {
			t.Fatalf("PostgreSQL fallback calls = %d, want 1", database.findByIDCalls)
		}
		cached, err := cache.Get(ctx, userID)
		if err != nil {
			t.Fatalf("get populated cache: %v", err)
		}
		assertUserAuthorization(t, cached, database.user)
	})

	t.Run("PostgreSQL failure is propagated", func(t *testing.T) {
		userID := uuid.NewString()
		registerUserCacheCleanup(t, client, userID)
		databaseErr := errors.New("PostgreSQL unavailable")
		database := &userRepositoryStub{findByIDErr: databaseErr}
		repository := NewCachedRepository(database, cache, &recordingUserCacheLogger{})

		role, err := repository.GetUserRole(ctx, userID)
		if !errors.Is(err, databaseErr) {
			t.Fatalf("GetUserRole() error = %v, want %v", err, databaseErr)
		}
		if role != valueobject.RoleUnknown {
			t.Fatalf("role = %d, want unknown on PostgreSQL failure", role)
		}
	})

	t.Run("missing authoritative user remains not found", func(t *testing.T) {
		userID := uuid.NewString()
		registerUserCacheCleanup(t, client, userID)
		repository := NewCachedRepository(&userRepositoryStub{}, cache, &recordingUserCacheLogger{})

		status, err := repository.GetUserStatus(ctx, userID)
		if !errors.Is(err, domainErrors.ErrUserNotFound) {
			t.Fatalf("GetUserStatus() error = %v, want user not found", err)
		}
		if status != valueobject.StatusUnknown {
			t.Fatalf("status = %d, want unknown for missing user", status)
		}
	})

	t.Run("cache miss remains distinct from authoritative PostgreSQL not found", func(t *testing.T) {
		userID := uuid.NewString()
		registerUserCacheCleanup(t, client, userID)
		postgresErr := fmt.Errorf("%w: %w", domainErrors.ErrUserNotFound, sql.ErrNoRows)
		database := &userRepositoryStub{findByIDErr: postgresErr}
		repository := NewCachedRepository(database, cache, &recordingUserCacheLogger{})

		role, err := repository.GetUserRole(ctx, userID)

		if !errors.Is(err, domainErrors.ErrUserNotFound) {
			t.Fatalf("GetUserRole() error = %v, want ErrUserNotFound", err)
		}
		if !errors.Is(err, sql.ErrNoRows) {
			t.Fatalf("GetUserRole() error = %v, want preserved sql.ErrNoRows", err)
		}
		if role != valueobject.RoleUnknown {
			t.Fatalf("role = %d, want unknown for authoritative not found", role)
		}
		if database.findByIDCalls != 1 {
			t.Fatalf("PostgreSQL fallback calls = %d, want 1", database.findByIDCalls)
		}
	})
}

func TestCachedRepositoryReportsRedisReadFailureAndUsesPostgreSQL(t *testing.T) {
	ctx, setupClient, _ := newUserCacheIntegration(t)
	userID := uuid.NewString()
	registerUserCacheCleanup(t, setupClient, userID)
	client := redis.NewClient(&redis.Options{Addr: setupClient.Options().Addr})
	t.Cleanup(func() { _ = client.Close() })
	if err := client.Ping(ctx).Err(); err != nil {
		t.Fatalf("ping Redis failure-test client: %v", err)
	}
	client.AddHook(failRedisCommandHook{command: "get", err: errors.New("forced Redis read failure")})

	user := &entity.User{
		ID: userID, Role: valueobject.RoleClient, Status: valueobject.StatusActive,
		CreatedAt: time.Date(2026, time.July, 23, 16, 0, 0, 0, time.UTC),
	}
	database := &userRepositoryStub{user: user}
	logger := &recordingUserCacheLogger{}
	repository := NewCachedRepository(database, NewCache(client), logger)

	status, err := repository.GetUserStatus(ctx, userID)
	if err != nil {
		t.Fatalf("get user status after Redis read failure: %v", err)
	}
	if status != valueobject.StatusActive {
		t.Fatalf("status = %d, want active PostgreSQL status", status)
	}
	if database.findByIDCalls != 1 {
		t.Fatalf("PostgreSQL fallback calls = %d, want 1", database.findByIDCalls)
	}
	if !logger.containsWarning("read user authorization cache failed") {
		t.Fatalf("warnings = %v, want Redis read warning", logger.warnings)
	}
	populated, err := NewCache(setupClient).Get(ctx, userID)
	if err != nil {
		t.Fatalf("get cache populated after Redis read failure: %v", err)
	}
	assertUserAuthorization(t, populated, user)
}

func TestCachedRepositoryReportsRedisWriteFailureAndUsesPostgreSQL(t *testing.T) {
	ctx, setupClient, _ := newUserCacheIntegration(t)
	userID := uuid.NewString()
	registerUserCacheCleanup(t, setupClient, userID)
	client := redis.NewClient(&redis.Options{Addr: setupClient.Options().Addr})
	t.Cleanup(func() { _ = client.Close() })
	if err := client.Ping(ctx).Err(); err != nil {
		t.Fatalf("ping Redis failure-test client: %v", err)
	}
	client.AddHook(failRedisCommandHook{command: "set", err: errors.New("forced Redis write failure")})

	user := &entity.User{
		ID: userID, Role: valueobject.RoleAdmin, Status: valueobject.StatusActive,
		CreatedAt: time.Date(2026, time.July, 23, 16, 0, 0, 0, time.UTC),
	}
	database := &userRepositoryStub{user: user}
	logger := &recordingUserCacheLogger{}
	repository := NewCachedRepository(database, NewCache(client), logger)

	role, err := repository.GetUserRole(ctx, userID)
	if err != nil {
		t.Fatalf("get user role after Redis write failure: %v", err)
	}
	if role != valueobject.RoleAdmin {
		t.Fatalf("role = %d, want admin PostgreSQL role", role)
	}
	if !logger.containsWarning("write user authorization cache failed") {
		t.Fatalf("warnings = %v, want Redis write warning", logger.warnings)
	}
	if logger.containsInfo("cached successfully") {
		t.Fatalf("infos = %v, must not report successful cache write", logger.infos)
	}
	exists, err := setupClient.Exists(ctx, pkg.RedisKey("user", "id", userID)).Result()
	if err != nil {
		t.Fatalf("check failed cache write: %v", err)
	}
	if exists != 0 {
		t.Fatal("user authorization cache exists after forced write failure")
	}
}

func TestUserAuthorizationCacheExpiryAndInvalidation(t *testing.T) {
	ctx, client, cache := newUserCacheIntegration(t)
	createdAt := time.Date(2026, time.July, 23, 16, 0, 0, 0, time.UTC)

	t.Run("expiry becomes cache miss", func(t *testing.T) {
		userID := uuid.NewString()
		registerUserCacheCleanup(t, client, userID)
		cacheWithShortTTL := NewCache(client)
		cacheWithShortTTL.ttl = 100 * time.Millisecond
		user := &entity.User{ID: userID, Role: valueobject.RoleClient, Status: valueobject.StatusActive, CreatedAt: createdAt}
		if err := cacheWithShortTTL.Set(ctx, userID, user); err != nil {
			t.Fatalf("set expiring user authorization: %v", err)
		}
		got, err := cacheWithShortTTL.Get(ctx, userID)
		if err != nil {
			t.Fatalf("get user authorization before expiry: %v", err)
		}
		assertUserAuthorization(t, got, user)

		deadline := time.Now().Add(3 * time.Second)
		for time.Now().Before(deadline) {
			got, err = cacheWithShortTTL.Get(ctx, userID)
			if err != nil {
				t.Fatalf("get expiring user authorization: %v", err)
			}
			if got == nil {
				return
			}
			time.Sleep(10 * time.Millisecond)
		}
		t.Fatal("user authorization cache key did not expire")
	})

	t.Run("service invalidation removes cache key", func(t *testing.T) {
		userID := uuid.NewString()
		registerUserCacheCleanup(t, client, userID)
		user := &entity.User{ID: userID, Role: valueobject.RoleOperator, Status: valueobject.StatusActive, CreatedAt: createdAt}
		if err := cache.Set(ctx, userID, user); err != nil {
			t.Fatalf("set cached user authorization: %v", err)
		}
		logger := &recordingUserCacheLogger{}
		repository := NewCachedRepository(&userRepositoryStub{}, cache, logger)
		if err := repository.ClearCache(ctx, userID); err != nil {
			t.Fatalf("clear cached user authorization: %v", err)
		}
		got, err := cache.Get(ctx, userID)
		if err != nil {
			t.Fatalf("get invalidated user authorization: %v", err)
		}
		if got != nil {
			t.Fatalf("invalidated user authorization = %#v, want nil", got)
		}
		if !logger.containsInfo("cleared successfully") {
			t.Fatalf("infos = %v, want successful invalidation", logger.infos)
		}
	})
}

func TestCachedRepositoryReportsRedisInvalidationFailure(t *testing.T) {
	client := redis.NewClient(&redis.Options{Addr: "127.0.0.1:1"})
	if err := client.Close(); err != nil {
		t.Fatalf("close Redis client: %v", err)
	}
	logger := &recordingUserCacheLogger{}
	repository := NewCachedRepository(&userRepositoryStub{}, NewCache(client), logger)

	err := repository.ClearCache(context.Background(), "11111111-1111-4111-8111-111111111111")
	if err == nil {
		t.Fatal("ClearCache() error = nil, want Redis invalidation error")
	}
	if !logger.containsError("clear user authorization cache failed") {
		t.Fatalf("errors = %v, want invalidation failure", logger.errors)
	}
	if logger.containsInfo("cleared successfully") {
		t.Fatalf("infos = %v, must not report successful invalidation", logger.infos)
	}
}

func newUserCacheIntegration(t *testing.T) (context.Context, *redis.Client, *Cache) {
	t.Helper()
	address := os.Getenv("GOAT_REDIS_ADDR")
	if address == "" {
		t.Skip("set GOAT_REDIS_ADDR to run Redis integration tests")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	t.Cleanup(cancel)
	client := redis.NewClient(&redis.Options{Addr: address})
	t.Cleanup(func() { _ = client.Close() })
	if err := client.Ping(ctx).Err(); err != nil {
		t.Fatalf("ping Redis: %v", err)
	}
	return ctx, client, NewCache(client)
}

func registerUserCacheCleanup(t *testing.T, client *redis.Client, userID string) {
	t.Helper()
	key := pkg.RedisKey("user", "id", userID)
	if err := client.Del(context.Background(), key).Err(); err != nil {
		t.Fatalf("clear user cache key before test: %v", err)
	}
	t.Cleanup(func() { _ = client.Del(context.Background(), key).Err() })
}

func assertUserAuthorization(t *testing.T, got *userAuthorization, want *entity.User) {
	t.Helper()
	if got == nil {
		t.Fatal("cached user authorization is nil")
	}
	if got.id != want.ID || got.role != want.Role || got.status != want.Status || !got.createdAt.Equal(want.CreatedAt) {
		t.Fatalf("cached authorization = %#v, want values from %#v", got, want)
	}
	switch {
	case got.updatedAt == nil && want.UpdatedAt == nil:
	case got.updatedAt == nil || want.UpdatedAt == nil:
		t.Fatalf("cached updated_at = %v, want %v", got.updatedAt, want.UpdatedAt)
	case !got.updatedAt.Equal(*want.UpdatedAt):
		t.Fatalf("cached updated_at = %v, want %v", got.updatedAt, want.UpdatedAt)
	}
}

type userRepositoryStub struct {
	user          *entity.User
	findByIDErr   error
	findByIDCalls int
}

func (r *userRepositoryStub) Create(context.Context, *entity.User) error {
	return nil
}

func (r *userRepositoryStub) ExistsByID(context.Context, string) (bool, error) {
	return r.user != nil, nil
}

func (r *userRepositoryStub) FindByID(context.Context, string) (*entity.User, error) {
	r.findByIDCalls++
	return r.user, r.findByIDErr
}

func (r *userRepositoryStub) FindByEmail(context.Context, string) (*entity.User, error) {
	return r.user, nil
}

func (r *userRepositoryStub) GetCredentialVersion(context.Context, string) (int64, error) {
	if r.user == nil {
		return 0, domainErrors.ErrUserNotFound
	}
	return r.user.CredentialVersion, nil
}

func (r *userRepositoryStub) UpdatePassword(
	context.Context,
	string,
	valueobject.Password,
) (int64, error) {
	return 0, nil
}

func (r *userRepositoryStub) Update(context.Context, *entity.User) error {
	return nil
}

func (r *userRepositoryStub) Delete(context.Context, string) error {
	return nil
}

func (r *userRepositoryStub) List(
	context.Context,
	int,
	int,
	repository.UserListFilter,
) ([]*entity.User, int64, error) {
	return nil, 0, nil
}

type recordingUserCacheLogger struct {
	infos    []string
	warnings []string
	errors   []string
}

func (l *recordingUserCacheLogger) Info(message string, _ ...any) {
	l.infos = append(l.infos, message)
}

func (l *recordingUserCacheLogger) Error(message string, _ ...any) {
	l.errors = append(l.errors, message)
}

func (l *recordingUserCacheLogger) Warn(message string, _ ...any) {
	l.warnings = append(l.warnings, message)
}

func (*recordingUserCacheLogger) Debug(string, ...any) {}
func (*recordingUserCacheLogger) Panic(string, ...any) {}

func (l *recordingUserCacheLogger) containsInfo(part string) bool {
	return containsUserCacheLog(l.infos, part)
}

func (l *recordingUserCacheLogger) containsWarning(part string) bool {
	return containsUserCacheLog(l.warnings, part)
}

func (l *recordingUserCacheLogger) containsError(part string) bool {
	return containsUserCacheLog(l.errors, part)
}

func containsUserCacheLog(messages []string, part string) bool {
	for _, message := range messages {
		if strings.Contains(message, part) {
			return true
		}
	}
	return false
}

type failRedisCommandHook struct {
	command string
	err     error
}

func (h failRedisCommandHook) DialHook(next redis.DialHook) redis.DialHook {
	return next
}

func (h failRedisCommandHook) ProcessHook(next redis.ProcessHook) redis.ProcessHook {
	return func(ctx context.Context, command redis.Cmder) error {
		if command.Name() == h.command {
			return h.err
		}
		return next(ctx, command)
	}
}

func (h failRedisCommandHook) ProcessPipelineHook(next redis.ProcessPipelineHook) redis.ProcessPipelineHook {
	return next
}
