package permcache

import (
	"context"
	"errors"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/motixo/goat-api/internal/domain/entity"
	"github.com/motixo/goat-api/internal/domain/valueobject"
	"github.com/motixo/goat-api/internal/pkg"
	"github.com/redis/go-redis/v9"
)

func TestPermissionCacheRoundTrip(t *testing.T) {
	ctx, client, cache := newPermissionCacheIntegration(t)
	roleID := int8(valueobject.RoleAdmin)
	key := pkg.RedisKey("perm", "role", roleID)
	createdAt := time.Date(2026, time.July, 23, 16, 0, 0, 0, time.UTC)
	want := []*entity.Permission{
		{
			ID:        "33333333-3333-4333-8333-333333333333",
			Role:      valueobject.RoleAdmin,
			Action:    valueobject.PermFullAccess,
			CreatedAt: createdAt,
		},
		{
			ID:        "44444444-4444-4444-8444-444444444444",
			Role:      valueobject.RoleAdmin,
			Action:    valueobject.PermUserDelete,
			CreatedAt: createdAt.Add(time.Second),
		},
	}

	if err := cache.Set(ctx, roleID, want); err != nil {
		t.Fatalf("set cached permissions: %v", err)
	}
	raw, err := client.Get(ctx, key).Result()
	if err != nil {
		t.Fatalf("get raw cached permissions: %v", err)
	}
	const wantJSON = `[{"id":"33333333-3333-4333-8333-333333333333","role":"admin","action":"full_access","created_at":"2026-07-23T16:00:00Z"},{"id":"44444444-4444-4444-8444-444444444444","role":"admin","action":"user:delete","created_at":"2026-07-23T16:00:01Z"}]`
	if raw != wantJSON {
		t.Fatalf("raw cached permissions = %s, want %s", raw, wantJSON)
	}

	got, err := cache.Get(ctx, roleID)
	if err != nil {
		t.Fatalf("get cached permissions: %v", err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("cached permissions = %#v, want %#v", got, want)
	}
}

func TestPermissionCacheMissingValueIsCacheMiss(t *testing.T) {
	ctx, _, cache := newPermissionCacheIntegration(t)

	got, err := cache.Get(ctx, int8(valueobject.RoleOperator))
	if err != nil {
		t.Fatalf("get missing cached permissions: %v", err)
	}
	if got != nil {
		t.Fatalf("missing cached permissions = %#v, want nil", got)
	}
}

func TestPermissionCacheRejectsMalformedAndIncompleteValues(t *testing.T) {
	ctx, client, cache := newPermissionCacheIntegration(t)
	roleID := int8(valueobject.RoleAdmin)
	key := pkg.RedisKey("perm", "role", roleID)
	tests := []struct {
		name    string
		payload string
	}{
		{name: "malformed JSON", payload: `[{`},
		{name: "null instead of array", payload: `null`},
		{
			name:    "incomplete record",
			payload: `[{"id":"id","role":"admin","action":"full_access"}]`,
		},
		{
			name:    "invalid action",
			payload: `[{"id":"id","role":"admin","action":"database:drop","created_at":"2026-07-23T16:00:00Z"}]`,
		},
		{
			name:    "record role differs from cache role",
			payload: `[{"id":"id","role":"client","action":"full_access","created_at":"2026-07-23T16:00:00Z"}]`,
		},
		{
			name:    "old numeric role payload",
			payload: `[{"ID":"id","Role":3,"Action":"full_access","CreatedAt":"2026-07-23T16:00:00Z"}]`,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if err := client.Set(ctx, key, test.payload, time.Minute).Err(); err != nil {
				t.Fatalf("set invalid cache payload: %v", err)
			}
			if _, err := cache.Get(ctx, roleID); err == nil {
				t.Fatal("Get() error = nil, want malformed cache error")
			}
		})
	}
}

func TestPermissionCacheExpires(t *testing.T) {
	ctx, _, cache := newPermissionCacheIntegration(t)
	cache.ttl = 100 * time.Millisecond
	roleID := int8(valueobject.RoleClient)
	permission := &entity.Permission{
		ID:        "11111111-1111-4111-8111-111111111111",
		Role:      valueobject.RoleClient,
		Action:    valueobject.PermUserRead,
		CreatedAt: time.Now().UTC(),
	}
	if err := cache.Set(ctx, roleID, []*entity.Permission{permission}); err != nil {
		t.Fatalf("set expiring cached permission: %v", err)
	}
	got, err := cache.Get(ctx, roleID)
	if err != nil {
		t.Fatalf("get cached permission before expiry: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("cached permissions before expiry = %#v, want one permission", got)
	}

	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		got, err = cache.Get(ctx, roleID)
		if err != nil {
			t.Fatalf("get expiring cached permission: %v", err)
		}
		if got == nil {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("permission cache key did not expire")
}

func TestCachedRepositoryFallsBackAfterMalformedCacheValue(t *testing.T) {
	ctx, client, cache := newPermissionCacheIntegration(t)
	role := valueobject.RoleAdmin
	key := pkg.RedisKey("perm", "role", int8(role))
	if err := client.Set(ctx, key, `[{`, time.Minute).Err(); err != nil {
		t.Fatalf("set malformed cache value: %v", err)
	}

	want := []*entity.Permission{{
		ID:        "33333333-3333-4333-8333-333333333333",
		Role:      role,
		Action:    valueobject.PermFullAccess,
		CreatedAt: time.Date(2026, time.July, 23, 16, 0, 0, 0, time.UTC),
	}}
	database := &permissionRepositoryStub{permissions: want}
	logger := &recordingPermissionCacheLogger{}
	repository := NewCachedRepository(database, cache, logger)

	got, err := repository.GetRolePermissions(ctx, role)
	if err != nil {
		t.Fatalf("get permissions after malformed cache value: %v", err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("permissions = %#v, want %#v", got, want)
	}
	if database.getCalls != 1 {
		t.Fatalf("database calls = %d, want 1", database.getCalls)
	}
	if !logger.containsWarning("read permission cache failed") {
		t.Fatalf("warnings = %v, want cache read failure", logger.warnings)
	}

	refreshed, err := cache.Get(ctx, int8(role))
	if err != nil {
		t.Fatalf("get refreshed cache value: %v", err)
	}
	if !reflect.DeepEqual(refreshed, want) {
		t.Fatalf("refreshed cache = %#v, want %#v", refreshed, want)
	}
}

func TestCachedRepositoryDoesNotSilentlyIgnoreRedisFailures(t *testing.T) {
	client := redis.NewClient(&redis.Options{Addr: "127.0.0.1:1"})
	if err := client.Close(); err != nil {
		t.Fatalf("close Redis client: %v", err)
	}
	cache := NewCache(client)
	want := []*entity.Permission{{
		ID:        "33333333-3333-4333-8333-333333333333",
		Role:      valueobject.RoleAdmin,
		Action:    valueobject.PermFullAccess,
		CreatedAt: time.Now().UTC(),
	}}
	database := &permissionRepositoryStub{permissions: want}
	logger := &recordingPermissionCacheLogger{}
	repository := NewCachedRepository(database, cache, logger)

	got, err := repository.GetRolePermissions(context.Background(), valueobject.RoleAdmin)
	if err != nil {
		t.Fatalf("get permissions with unavailable Redis: %v", err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("permissions = %#v, want %#v", got, want)
	}
	if database.getCalls != 1 {
		t.Fatalf("database calls = %d, want 1", database.getCalls)
	}
	if !logger.containsWarning("read permission cache failed") ||
		!logger.containsWarning("write permission cache failed") {
		t.Fatalf("warnings = %v, want read and write failures", logger.warnings)
	}
}

func newPermissionCacheIntegration(t *testing.T) (context.Context, *redis.Client, *Cache) {
	t.Helper()
	address := os.Getenv("GOAT_REDIS_ADDR")
	if address == "" {
		t.Skip("set GOAT_REDIS_ADDR to run Redis integration tests")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	t.Cleanup(cancel)
	client := redis.NewClient(&redis.Options{Addr: address})
	t.Cleanup(func() { _ = client.Close() })
	if err := client.Ping(ctx).Err(); err != nil {
		t.Fatalf("ping Redis: %v", err)
	}

	keys := make([]string, 0, len(valueobject.AllRoles()))
	for _, role := range valueobject.AllRoles() {
		keys = append(keys, pkg.RedisKey("perm", "role", int8(role)))
	}
	if err := client.Del(ctx, keys...).Err(); err != nil {
		t.Fatalf("clear permission cache keys: %v", err)
	}
	t.Cleanup(func() { _ = client.Del(context.Background(), keys...).Err() })
	return ctx, client, NewCache(client)
}

type permissionRepositoryStub struct {
	permissions []*entity.Permission
	getErr      error
	getCalls    int
}

func (r *permissionRepositoryStub) Create(context.Context, *entity.Permission) error {
	return nil
}

func (r *permissionRepositoryStub) List(context.Context, int, int) ([]*entity.Permission, int64, error) {
	return nil, 0, nil
}

func (r *permissionRepositoryStub) GetByRoleID(
	context.Context,
	valueobject.UserRole,
) ([]*entity.Permission, error) {
	r.getCalls++
	return r.permissions, r.getErr
}

func (r *permissionRepositoryStub) Delete(context.Context, string) (int8, error) {
	return 0, errors.New("not implemented")
}

type recordingPermissionCacheLogger struct {
	warnings []string
}

func (l *recordingPermissionCacheLogger) Info(string, ...any)  {}
func (l *recordingPermissionCacheLogger) Error(string, ...any) {}
func (l *recordingPermissionCacheLogger) Debug(string, ...any) {}
func (l *recordingPermissionCacheLogger) Panic(string, ...any) {}

func (l *recordingPermissionCacheLogger) Warn(message string, _ ...any) {
	l.warnings = append(l.warnings, message)
}

func (l *recordingPermissionCacheLogger) containsWarning(part string) bool {
	for _, warning := range l.warnings {
		if strings.Contains(warning, part) {
			return true
		}
	}
	return false
}
