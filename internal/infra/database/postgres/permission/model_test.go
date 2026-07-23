package permission

import (
	"reflect"
	"testing"
	"time"

	"github.com/motixo/goat-api/internal/domain/entity"
	"github.com/motixo/goat-api/internal/domain/valueobject"
)

func TestPermissionRowToDomainPreservesAllFields(t *testing.T) {
	createdAt := time.Date(2026, time.July, 23, 14, 30, 0, 123000000, time.UTC)
	row := permissionRow{
		ID:        "11111111-1111-4111-8111-111111111111",
		Role:      int16(valueobject.RoleOperator),
		Action:    valueobject.PermUserUpdate.String(),
		CreatedAt: createdAt,
	}

	got := row.toDomain()
	want := &entity.Permission{
		ID:        row.ID,
		Role:      valueobject.RoleOperator,
		Action:    valueobject.PermUserUpdate,
		CreatedAt: createdAt,
	}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("toDomain() = %#v, want %#v", got, want)
	}
}

func TestPermissionRowFromDomainPreservesAllFields(t *testing.T) {
	createdAt := time.Date(2026, time.July, 23, 15, 0, 0, 456000000, time.UTC)
	permission := &entity.Permission{
		ID:        "22222222-2222-4222-8222-222222222222",
		Role:      valueobject.RoleAdmin,
		Action:    valueobject.PermFullAccess,
		CreatedAt: createdAt,
	}

	got := permissionRowFromDomain(permission)
	want := permissionRow{
		ID:        permission.ID,
		Role:      int16(valueobject.RoleAdmin),
		Action:    valueobject.PermFullAccess.String(),
		CreatedAt: createdAt,
	}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("permissionRowFromDomain() = %#v, want %#v", got, want)
	}
}

func TestPermissionRowsToDomainPreservesOrder(t *testing.T) {
	rows := []permissionRow{
		{ID: "permission-1", Role: int16(valueobject.RoleAdmin), Action: valueobject.PermFullAccess.String()},
		{ID: "permission-2", Role: int16(valueobject.RoleOperator), Action: valueobject.PermUserRead.String()},
	}

	permissions := permissionRowsToDomain(rows)
	if len(permissions) != len(rows) {
		t.Fatalf("permission count = %d, want %d", len(permissions), len(rows))
	}
	for index := range rows {
		if permissions[index].ID != rows[index].ID {
			t.Fatalf("permissions[%d].ID = %q, want %q", index, permissions[index].ID, rows[index].ID)
		}
	}
}
