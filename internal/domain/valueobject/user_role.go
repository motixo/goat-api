package valueobject

import (
	"fmt"
)

type UserRole uint8

const (
	RoleUnknown UserRole = iota
	RoleClient
	RoleOperator
	RoleAdmin
)

var roleToString = map[UserRole]string{
	RoleClient:   "client",
	RoleOperator: "operator",
	RoleAdmin:    "admin",
}

var stringToRole = map[string]UserRole{
	"client":   RoleClient,
	"operator": RoleOperator,
	"admin":    RoleAdmin,
}

func (r UserRole) String() string {
	s, ok := roleToString[r]
	if !ok {
		return "unknown"
	}
	return s
}

func ParseUserRole(s string) (UserRole, error) {
	r, ok := stringToRole[s]
	if !ok {
		return 0, fmt.Errorf("invalid user role: %s", s)
	}
	return r, nil
}

func AllRoles() []UserRole {
	return []UserRole{
		RoleClient,
		RoleOperator,
		RoleAdmin,
	}
}

func (r UserRole) CanModifyTargetRole(target UserRole) bool {
	switch r {
	case RoleAdmin:
		return true
	case RoleOperator:
		return target == RoleClient
	default:
		return false
	}
}

func VisibleRoles(actor UserRole) []UserRole {
	switch actor {
	case RoleAdmin:
		return []UserRole{RoleAdmin, RoleOperator, RoleClient}
	case RoleOperator:
		return []UserRole{RoleClient}
	default:
		return []UserRole{}
	}
}
