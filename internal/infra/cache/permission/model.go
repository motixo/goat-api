package permcache

import (
	"fmt"
	"strings"
	"time"

	"github.com/motixo/goat-api/internal/domain/entity"
	"github.com/motixo/goat-api/internal/domain/valueobject"
)

type permissionCacheRecord struct {
	ID        string    `json:"id"`
	Role      string    `json:"role"`
	Action    string    `json:"action"`
	CreatedAt time.Time `json:"created_at"`
}

func permissionCacheRecordFromDomain(permission *entity.Permission) (permissionCacheRecord, error) {
	if permission == nil {
		return permissionCacheRecord{}, fmt.Errorf("permission is nil")
	}
	if strings.TrimSpace(permission.ID) == "" {
		return permissionCacheRecord{}, fmt.Errorf("permission ID is empty")
	}
	if err := validatePermissionCacheRole(permission.Role); err != nil {
		return permissionCacheRecord{}, err
	}
	if _, err := valueobject.ParsePermission(permission.Action.String()); err != nil {
		return permissionCacheRecord{}, fmt.Errorf("permission action: %w", err)
	}
	if permission.CreatedAt.IsZero() {
		return permissionCacheRecord{}, fmt.Errorf("permission created_at is zero")
	}

	return permissionCacheRecord{
		ID:        permission.ID,
		Role:      permission.Role.String(),
		Action:    permission.Action.String(),
		CreatedAt: permission.CreatedAt,
	}, nil
}

func permissionCacheRecordsFromDomain(
	permissions []*entity.Permission,
	expectedRole valueobject.UserRole,
) ([]permissionCacheRecord, error) {
	if err := validatePermissionCacheRole(expectedRole); err != nil {
		return nil, err
	}

	records := make([]permissionCacheRecord, 0, len(permissions))
	for index, permission := range permissions {
		record, err := permissionCacheRecordFromDomain(permission)
		if err != nil {
			return nil, fmt.Errorf("permission at index %d: %w", index, err)
		}
		if permission.Role != expectedRole {
			return nil, fmt.Errorf(
				"permission at index %d has role %q, expected %q",
				index,
				permission.Role.String(),
				expectedRole.String(),
			)
		}
		records = append(records, record)
	}
	return records, nil
}

func (record permissionCacheRecord) toDomain(expectedRole valueobject.UserRole) (*entity.Permission, error) {
	if strings.TrimSpace(record.ID) == "" {
		return nil, fmt.Errorf("permission ID is empty")
	}
	if err := validatePermissionCacheRole(expectedRole); err != nil {
		return nil, err
	}

	role, err := valueobject.ParseUserRole(record.Role)
	if err != nil {
		return nil, fmt.Errorf("permission role: %w", err)
	}
	if role != expectedRole {
		return nil, fmt.Errorf(
			"permission role %q does not match cache role %q",
			record.Role,
			expectedRole.String(),
		)
	}

	action, err := valueobject.ParsePermission(record.Action)
	if err != nil {
		return nil, fmt.Errorf("permission action: %w", err)
	}
	if record.CreatedAt.IsZero() {
		return nil, fmt.Errorf("permission created_at is zero")
	}

	return &entity.Permission{
		ID:        record.ID,
		Role:      role,
		Action:    action,
		CreatedAt: record.CreatedAt,
	}, nil
}

func permissionCacheRecordsToDomain(
	records []permissionCacheRecord,
	expectedRole valueobject.UserRole,
) ([]*entity.Permission, error) {
	if records == nil {
		return nil, fmt.Errorf("permission cache payload must be a JSON array")
	}

	permissions := make([]*entity.Permission, 0, len(records))
	for index := range records {
		permission, err := records[index].toDomain(expectedRole)
		if err != nil {
			return nil, fmt.Errorf("permission at index %d: %w", index, err)
		}
		permissions = append(permissions, permission)
	}
	return permissions, nil
}

func permissionCacheRole(roleID int8) (valueobject.UserRole, error) {
	role := valueobject.UserRole(roleID)
	if err := validatePermissionCacheRole(role); err != nil {
		return valueobject.RoleUnknown, err
	}
	return role, nil
}

func validatePermissionCacheRole(role valueobject.UserRole) error {
	parsedRole, err := valueobject.ParseUserRole(role.String())
	if err != nil || parsedRole != role {
		return fmt.Errorf("invalid permission cache role: %d", role)
	}
	return nil
}
