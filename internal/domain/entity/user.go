package entity

import (
	"time"

	"github.com/mot0x0/goth-api/internal/domain/valueobject"
)

type User struct {
	ID        string                 `json:"id" db:"id"`
	Email     string                 `json:"email" db:"email"`
	Password  string                 `json:"-" db:"password"`
	Status    valueobject.UserStatus `json:"status" db:"status"`
	CreatedAt time.Time              `json:"created_at" db:"created_at"`
	UpdatedAt *time.Time             `json:"updated_at,omitempty" db:"updated_at"`
}
