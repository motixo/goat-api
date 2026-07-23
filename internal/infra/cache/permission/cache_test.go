package permcache

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/motixo/goat-api/internal/domain/entity"
	"github.com/motixo/goat-api/internal/domain/valueobject"
)

func TestPermissionCacheRecordMapping(t *testing.T) {
	createdAt := time.Date(2026, time.July, 23, 16, 0, 0, 0, time.UTC)
	domainPermission := &entity.Permission{
		ID:        "33333333-3333-4333-8333-333333333333",
		Role:      valueobject.RoleAdmin,
		Action:    valueobject.PermFullAccess,
		CreatedAt: createdAt,
	}

	record, err := permissionCacheRecordFromDomain(domainPermission)
	if err != nil {
		t.Fatalf("map domain permission to cache record: %v", err)
	}
	wantRecord := permissionCacheRecord{
		ID:        domainPermission.ID,
		Role:      "admin",
		Action:    "full_access",
		CreatedAt: createdAt,
	}
	if !reflect.DeepEqual(record, wantRecord) {
		t.Fatalf("cache record = %#v, want %#v", record, wantRecord)
	}

	mapped, err := record.toDomain(valueobject.RoleAdmin)
	if err != nil {
		t.Fatalf("map cache record to domain permission: %v", err)
	}
	if !reflect.DeepEqual(mapped, domainPermission) {
		t.Fatalf("domain permission = %#v, want %#v", mapped, domainPermission)
	}
}

func TestPermissionCacheRecordJSONContract(t *testing.T) {
	permission := &entity.Permission{
		ID:        "33333333-3333-4333-8333-333333333333",
		Role:      valueobject.RoleAdmin,
		Action:    valueobject.PermFullAccess,
		CreatedAt: time.Date(2026, time.July, 23, 16, 0, 0, 0, time.UTC),
	}
	records, err := permissionCacheRecordsFromDomain(
		[]*entity.Permission{permission},
		valueobject.RoleAdmin,
	)
	if err != nil {
		t.Fatalf("map permissions to cache records: %v", err)
	}

	encoded, err := json.Marshal(records)
	if err != nil {
		t.Fatalf("marshal permission cache records: %v", err)
	}
	const wantJSON = `[{"id":"33333333-3333-4333-8333-333333333333","role":"admin","action":"full_access","created_at":"2026-07-23T16:00:00Z"}]`
	if string(encoded) != wantJSON {
		t.Fatalf("cached JSON = %s, want %s", encoded, wantJSON)
	}
}

func TestPermissionCacheRecordRejectsInvalidValues(t *testing.T) {
	createdAt := time.Date(2026, time.July, 23, 16, 0, 0, 0, time.UTC)
	tests := []struct {
		name        string
		record      permissionCacheRecord
		role        valueobject.UserRole
		wantErrPart string
	}{
		{
			name: "missing ID",
			record: permissionCacheRecord{
				Role: "admin", Action: "full_access", CreatedAt: createdAt,
			},
			role:        valueobject.RoleAdmin,
			wantErrPart: "ID is empty",
		},
		{
			name: "invalid role",
			record: permissionCacheRecord{
				ID: "id", Role: "superadmin", Action: "full_access", CreatedAt: createdAt,
			},
			role:        valueobject.RoleAdmin,
			wantErrPart: "invalid user role",
		},
		{
			name: "role does not match cache key",
			record: permissionCacheRecord{
				ID: "id", Role: "client", Action: "full_access", CreatedAt: createdAt,
			},
			role:        valueobject.RoleAdmin,
			wantErrPart: "does not match cache role",
		},
		{
			name: "invalid action",
			record: permissionCacheRecord{
				ID: "id", Role: "admin", Action: "database:drop", CreatedAt: createdAt,
			},
			role:        valueobject.RoleAdmin,
			wantErrPart: "invalid permission",
		},
		{
			name: "missing creation time",
			record: permissionCacheRecord{
				ID: "id", Role: "admin", Action: "full_access",
			},
			role:        valueobject.RoleAdmin,
			wantErrPart: "created_at is zero",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := test.record.toDomain(test.role)
			if err == nil || !strings.Contains(err.Error(), test.wantErrPart) {
				t.Fatalf("toDomain() error = %v, want error containing %q", err, test.wantErrPart)
			}
		})
	}
}

func TestPermissionCacheRecordsFromDomainRejectsInvalidValues(t *testing.T) {
	valid := &entity.Permission{
		ID:        "id",
		Role:      valueobject.RoleAdmin,
		Action:    valueobject.PermFullAccess,
		CreatedAt: time.Now().UTC(),
	}

	tests := []struct {
		name        string
		permissions []*entity.Permission
		wantErrPart string
	}{
		{name: "nil permission", permissions: []*entity.Permission{nil}, wantErrPart: "permission is nil"},
		{
			name: "wrong role",
			permissions: []*entity.Permission{{
				ID: valid.ID, Role: valueobject.RoleClient, Action: valid.Action, CreatedAt: valid.CreatedAt,
			}},
			wantErrPart: "expected \"admin\"",
		},
		{
			name: "unknown action",
			permissions: []*entity.Permission{{
				ID: valid.ID, Role: valid.Role, Action: valueobject.Permission("unknown"), CreatedAt: valid.CreatedAt,
			}},
			wantErrPart: "invalid permission",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := permissionCacheRecordsFromDomain(test.permissions, valueobject.RoleAdmin)
			if err == nil || !strings.Contains(err.Error(), test.wantErrPart) {
				t.Fatalf("mapping error = %v, want error containing %q", err, test.wantErrPart)
			}
		})
	}
}

func TestPermissionCacheEmptyCollectionUsesJSONArray(t *testing.T) {
	records, err := permissionCacheRecordsFromDomain(nil, valueobject.RoleClient)
	if err != nil {
		t.Fatalf("map empty permissions: %v", err)
	}
	encoded, err := json.Marshal(records)
	if err != nil {
		t.Fatalf("marshal empty records: %v", err)
	}
	if string(encoded) != "[]" {
		t.Fatalf("empty cache payload = %s, want []", encoded)
	}

	permissions, err := permissionCacheRecordsToDomain(records, valueobject.RoleClient)
	if err != nil {
		t.Fatalf("map empty records: %v", err)
	}
	if permissions == nil || len(permissions) != 0 {
		t.Fatalf("empty permissions = %#v, want non-nil empty slice", permissions)
	}
}
