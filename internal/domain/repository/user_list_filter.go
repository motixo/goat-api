package repository

import "github.com/motixo/goat-api/internal/domain/valueobject"

// UserListFilter defines the criteria supported by UserRepository's list
// operation. It deliberately excludes transport and database-specific details.
type UserListFilter struct {
	Statuses []valueobject.UserStatus
	Roles    []valueobject.UserRole
	Search   string
}
