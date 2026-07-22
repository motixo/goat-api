package entity

import (
	"time"

	"github.com/motixo/goat-api/internal/domain/valueobject"
)

type User struct {
	ID        string
	Email     string
	Password  valueobject.Password
	Status    valueobject.UserStatus
	Role      valueobject.UserRole
	CreatedAt time.Time
	UpdatedAt *time.Time
}
