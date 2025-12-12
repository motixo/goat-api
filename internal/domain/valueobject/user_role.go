package valueobject

import (
	"encoding/json"
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

// UnmarshalJSON allows parsing string roles from JSON
func (r *UserRole) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return fmt.Errorf("user role must be a string")
	}

	parsedRole, err := ParseUserRole(s)
	if err != nil {
		return err
	}

	*r = parsedRole
	return nil
}
