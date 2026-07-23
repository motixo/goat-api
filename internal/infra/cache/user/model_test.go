package usercache

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/motixo/goat-api/internal/domain/entity"
	"github.com/motixo/goat-api/internal/domain/valueobject"
)

func TestUserCacheRecordMapping(t *testing.T) {
	createdAt := time.Date(2026, time.July, 23, 16, 0, 0, 0, time.UTC)
	updatedAt := createdAt.Add(time.Hour)
	domainUser := &entity.User{
		ID:        "11111111-1111-4111-8111-111111111111",
		Role:      valueobject.RoleAdmin,
		Status:    valueobject.StatusActive,
		CreatedAt: createdAt,
		UpdatedAt: &updatedAt,
	}

	record, err := userCacheRecordFromDomain(domainUser, domainUser.ID)
	if err != nil {
		t.Fatalf("map domain user to cache record: %v", err)
	}
	wantRecord := userCacheRecord{
		ID:        domainUser.ID,
		Role:      "admin",
		Status:    "active",
		CreatedAt: createdAt,
		UpdatedAt: &updatedAt,
	}
	if !reflect.DeepEqual(record, wantRecord) {
		t.Fatalf("cache record = %#v, want %#v", record, wantRecord)
	}

	authorization, err := record.toAuthorization(domainUser.ID)
	if err != nil {
		t.Fatalf("map cache record to authorization values: %v", err)
	}
	if authorization.id != domainUser.ID ||
		authorization.role != domainUser.Role ||
		authorization.status != domainUser.Status ||
		!authorization.createdAt.Equal(domainUser.CreatedAt) ||
		authorization.updatedAt == nil ||
		!authorization.updatedAt.Equal(updatedAt) {
		t.Fatalf("authorization = %#v, want values from %#v", authorization, domainUser)
	}
}

func TestUserCacheRecordJSONContract(t *testing.T) {
	createdAt := time.Date(2026, time.July, 23, 16, 0, 0, 0, time.UTC)
	updatedAt := createdAt.Add(time.Hour)
	record, err := userCacheRecordFromDomain(&entity.User{
		ID:        "11111111-1111-4111-8111-111111111111",
		Role:      valueobject.RoleAdmin,
		Status:    valueobject.StatusActive,
		CreatedAt: createdAt,
		UpdatedAt: &updatedAt,
	}, "11111111-1111-4111-8111-111111111111")
	if err != nil {
		t.Fatalf("map cache record: %v", err)
	}

	encoded, err := json.Marshal(record)
	if err != nil {
		t.Fatalf("marshal user cache record: %v", err)
	}
	const wantJSON = `{"id":"11111111-1111-4111-8111-111111111111","role":"admin","status":"active","created_at":"2026-07-23T16:00:00Z","updated_at":"2026-07-23T17:00:00Z"}`
	if string(encoded) != wantJSON {
		t.Fatalf("cached JSON = %s, want %s", encoded, wantJSON)
	}
}

func TestUserCacheRecordRejectsInvalidValues(t *testing.T) {
	createdAt := time.Date(2026, time.July, 23, 16, 0, 0, 0, time.UTC)
	zeroTime := time.Time{}
	beforeCreation := createdAt.Add(-time.Second)
	const expectedUserID = "11111111-1111-4111-8111-111111111111"
	tests := []struct {
		name        string
		record      userCacheRecord
		expectedID  string
		wantErrPart string
	}{
		{
			name: "missing record ID",
			record: userCacheRecord{
				Role: "client", Status: "active", CreatedAt: createdAt,
			},
			expectedID:  expectedUserID,
			wantErrPart: "user ID is empty",
		},
		{
			name: "malformed record ID",
			record: userCacheRecord{
				ID: "not-a-uuid", Role: "client", Status: "active", CreatedAt: createdAt,
			},
			expectedID:  expectedUserID,
			wantErrPart: "invalid user ID",
		},
		{
			name: "mismatched record identity",
			record: userCacheRecord{
				ID: "22222222-2222-4222-8222-222222222222", Role: "client", Status: "active", CreatedAt: createdAt,
			},
			expectedID:  expectedUserID,
			wantErrPart: "does not match cache key identity",
		},
		{
			name: "unknown role",
			record: userCacheRecord{
				ID: expectedUserID, Role: "owner", Status: "active", CreatedAt: createdAt,
			},
			expectedID:  expectedUserID,
			wantErrPart: "cached user role",
		},
		{
			name: "unknown status",
			record: userCacheRecord{
				ID: expectedUserID, Role: "client", Status: "deleted", CreatedAt: createdAt,
			},
			expectedID:  expectedUserID,
			wantErrPart: "cached user status",
		},
		{
			name: "missing creation time",
			record: userCacheRecord{
				ID: expectedUserID, Role: "client", Status: "active",
			},
			expectedID:  expectedUserID,
			wantErrPart: "created_at is zero",
		},
		{
			name: "zero update time",
			record: userCacheRecord{
				ID: expectedUserID, Role: "client", Status: "active", CreatedAt: createdAt, UpdatedAt: &zeroTime,
			},
			expectedID:  expectedUserID,
			wantErrPart: "updated_at is zero",
		},
		{
			name: "update before creation",
			record: userCacheRecord{
				ID: expectedUserID, Role: "client", Status: "active", CreatedAt: createdAt, UpdatedAt: &beforeCreation,
			},
			expectedID:  expectedUserID,
			wantErrPart: "precedes created_at",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := test.record.toAuthorization(test.expectedID)
			if err == nil || !strings.Contains(err.Error(), test.wantErrPart) {
				t.Fatalf("toAuthorization() error = %v, want error containing %q", err, test.wantErrPart)
			}
		})
	}
}

func TestUserCacheDomainMappingRejectsInvalidValues(t *testing.T) {
	createdAt := time.Date(2026, time.July, 23, 16, 0, 0, 0, time.UTC)
	const expectedUserID = "11111111-1111-4111-8111-111111111111"
	validUser := func() *entity.User {
		return &entity.User{
			ID: expectedUserID, Role: valueobject.RoleClient, Status: valueobject.StatusActive, CreatedAt: createdAt,
		}
	}
	tests := []struct {
		name        string
		user        func() *entity.User
		expectedID  string
		wantErrPart string
	}{
		{name: "nil user", user: func() *entity.User { return nil }, expectedID: expectedUserID, wantErrPart: "user is nil"},
		{
			name: "mismatched identity",
			user: func() *entity.User {
				user := validUser()
				user.ID = "22222222-2222-4222-8222-222222222222"
				return user
			},
			expectedID:  expectedUserID,
			wantErrPart: "does not match cache key identity",
		},
		{
			name: "unknown domain role",
			user: func() *entity.User {
				user := validUser()
				user.Role = valueobject.UserRole(99)
				return user
			},
			expectedID:  expectedUserID,
			wantErrPart: "invalid user cache role",
		},
		{
			name: "unknown domain status",
			user: func() *entity.User {
				user := validUser()
				user.Status = valueobject.UserStatus(99)
				return user
			},
			expectedID:  expectedUserID,
			wantErrPart: "invalid user cache status",
		},
		{
			name: "missing creation time",
			user: func() *entity.User {
				user := validUser()
				user.CreatedAt = time.Time{}
				return user
			},
			expectedID:  expectedUserID,
			wantErrPart: "created_at is zero",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := userCacheRecordFromDomain(test.user(), test.expectedID)
			if err == nil || !strings.Contains(err.Error(), test.wantErrPart) {
				t.Fatalf("mapping error = %v, want error containing %q", err, test.wantErrPart)
			}
		})
	}
}
