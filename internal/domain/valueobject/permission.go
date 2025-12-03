package valueobject

type Permission string

const (
	// User
	PermUserRead         Permission = "user:read"
	PermUserWrite        Permission = "user:write"
	PermUserUpdate       Permission = "user:update"
	PermUserDelete       Permission = "user:delete"
	PermUserChangeRole   Permission = "user:change_role"
	PermUserChangeStatus Permission = "user:change_status"

	// Session
	PermSessionRead   Permission = "session:read"
	PermSessionDelete Permission = "session:delete"
)
