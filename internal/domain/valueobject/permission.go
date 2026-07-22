package valueobject

import (
	"fmt"
)

type Permission string

const (

	// Full access
	PermFullAccess Permission = "full_access"

	// User
	PermUserRead         Permission = "user:read"
	PermUserWrite        Permission = "user:write"
	PermUserUpdate       Permission = "user:update"
	PermUserDelete       Permission = "user:delete"
	PermUserChangeRole   Permission = "user:change_role"
	PermUserChangeStatus Permission = "user:change_status"
)

var knownPermissions = map[Permission]struct{}{
	PermFullAccess:       {},
	PermUserRead:         {},
	PermUserWrite:        {},
	PermUserUpdate:       {},
	PermUserDelete:       {},
	PermUserChangeRole:   {},
	PermUserChangeStatus: {},
}

func ParsePermission(s string) (Permission, error) {
	perm := Permission(s)
	if _, ok := knownPermissions[perm]; !ok {
		return "", fmt.Errorf("invalid permission: %q", s)
	}
	return perm, nil
}

func (p Permission) String() string {
	return string(p)
}
