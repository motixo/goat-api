package valueobject

type UserRole uint8

const (
	RoleClient UserRole = iota
	RoleOperator
	RoleAdmin
)

func (r UserRole) String() string {
	return [...]string{"client", "operator", "admin"}[r]
}
