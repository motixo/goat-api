package main

import (
	"context"
	"reflect"
	"sync"
	"testing"
	"time"

	"github.com/motixo/goat-api/internal/config"
	"github.com/motixo/goat-api/internal/delivery/http/middleware"
	"github.com/motixo/goat-api/internal/domain/entity"
	domainEvent "github.com/motixo/goat-api/internal/domain/event"
	"github.com/motixo/goat-api/internal/domain/valueobject"
)

func TestNewRateLimitConfig(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		RateLimitAuthLimit:     5,
		RateLimitAuthWindow:    time.Minute,
		RateLimitPublicLimit:   100,
		RateLimitPublicWindow:  2 * time.Minute,
		RateLimitPrivateLimit:  60,
		RateLimitPrivateWindow: 3 * time.Minute,
	}

	want := middleware.RateLimitConfig{
		Auth: middleware.RateLimit{
			Limit:  5,
			Window: time.Minute,
		},
		Public: middleware.RateLimit{
			Limit:  100,
			Window: 2 * time.Minute,
		},
		Private: middleware.RateLimit{
			Limit:  60,
			Window: 3 * time.Minute,
		},
	}

	if got := newRateLimitConfig(cfg); !reflect.DeepEqual(got, want) {
		t.Fatalf("newRateLimitConfig() = %#v, want %#v", got, want)
	}
}

func TestNewConfiguredEventBusRegistersCacheInvalidationHandlers(t *testing.T) {
	t.Parallel()

	userCache := &recordingUserCache{}
	permissionCache := &recordingPermissionCache{}
	bus := newConfiguredEventBus(discardLogger{}, userCache, permissionCache)

	if err := bus.Publish(context.Background(), domainEvent.UserUpdatedEvent{UserID: "user-1"}); err != nil {
		t.Fatalf("publish user event: %v", err)
	}
	if err := bus.Publish(context.Background(), domainEvent.PermissionUpdatedEvent{Role: valueobject.RoleAdmin}); err != nil {
		t.Fatalf("publish permission event: %v", err)
	}
	bus.Wait()

	if got := userCache.clearedUserIDs(); !reflect.DeepEqual(got, []string{"user-1"}) {
		t.Fatalf("cleared user IDs = %v, want [user-1]", got)
	}
	if got := permissionCache.clearedRoles(); !reflect.DeepEqual(got, []valueobject.UserRole{valueobject.RoleAdmin}) {
		t.Fatalf("cleared permission roles = %v, want [%v]", got, valueobject.RoleAdmin)
	}
}

type recordingUserCache struct {
	mu      sync.Mutex
	userIDs []string
}

func (c *recordingUserCache) GetUserStatus(context.Context, string) (valueobject.UserStatus, error) {
	return valueobject.StatusUnknown, nil
}

func (c *recordingUserCache) GetUserRole(context.Context, string) (valueobject.UserRole, error) {
	return valueobject.RoleUnknown, nil
}

func (c *recordingUserCache) ClearCache(_ context.Context, userID string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.userIDs = append(c.userIDs, userID)
	return nil
}

func (c *recordingUserCache) clearedUserIDs() []string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]string(nil), c.userIDs...)
}

type recordingPermissionCache struct {
	mu    sync.Mutex
	roles []valueobject.UserRole
}

func (c *recordingPermissionCache) GetRolePermissions(context.Context, valueobject.UserRole) ([]*entity.Permission, error) {
	return nil, nil
}

func (c *recordingPermissionCache) ClearCache(_ context.Context, role valueobject.UserRole) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.roles = append(c.roles, role)
	return nil
}

func (c *recordingPermissionCache) clearedRoles() []valueobject.UserRole {
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]valueobject.UserRole(nil), c.roles...)
}

type discardLogger struct{}

func (discardLogger) Info(string, ...any)  {}
func (discardLogger) Error(string, ...any) {}
func (discardLogger) Warn(string, ...any)  {}
func (discardLogger) Debug(string, ...any) {}
func (discardLogger) Panic(string, ...any) {}
