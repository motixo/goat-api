package valueobject

import (
	"fmt"
	"sort"
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

var allPermissions = []Permission{
	PermFullAccess,
	PermUserChangeRole,
	PermUserChangeStatus,
	PermUserDelete,
	PermUserRead,
	PermUserUpdate,
	PermUserWrite,
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

// AllPermissions returns the complete bounded permission vocabulary in stable
// lexical order.
func AllPermissions() []Permission {
	return append([]Permission(nil), allPermissions...)
}

// PermissionSet is a validated, normalized authorization snapshot. Its values
// are kept private so callers cannot mutate a verified principal through a
// shared slice.
type PermissionSet struct {
	values []Permission
}

func NewPermissionSet(permissions []Permission) (PermissionSet, error) {
	unique := make(map[Permission]struct{}, len(permissions))
	for _, permission := range permissions {
		if _, ok := knownPermissions[permission]; !ok {
			return PermissionSet{}, fmt.Errorf("invalid permission: %q", permission)
		}
		unique[permission] = struct{}{}
	}
	values := make([]Permission, 0, len(unique))
	for permission := range unique {
		values = append(values, permission)
	}
	sort.Slice(values, func(i, j int) bool {
		return values[i] < values[j]
	})
	return PermissionSet{values: values}, nil
}

func (s PermissionSet) Values() []Permission {
	return append([]Permission(nil), s.values...)
}

func (s PermissionSet) Has(required Permission) bool {
	if _, ok := knownPermissions[required]; !ok {
		return false
	}
	for _, permission := range s.values {
		if permission == required || permission == PermFullAccess {
			return true
		}
	}
	return false
}
