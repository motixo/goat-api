package entity

import (
	"github.com/motixo/goat-api/internal/domain/valueobject"
)

type UserFilter struct {
	Statuses []valueobject.UserStatus
	Roles    []valueobject.UserRole
	Search   string
}
