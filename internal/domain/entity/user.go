package entity

import (
	"time"

	"github.com/motixo/goat-api/internal/domain/valueobject"
)

const InitialCredentialVersion int64 = 1

type User struct {
	ID                string
	Email             string
	Password          valueobject.Password
	Status            valueobject.UserStatus
	Role              valueobject.UserRole
	CredentialVersion int64
	CreatedAt         time.Time
	UpdatedAt         *time.Time
}
