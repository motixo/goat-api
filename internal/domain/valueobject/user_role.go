package valueobject

import "fmt"

type UserRole uint8

const (
	RoleClient UserRole = iota
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
