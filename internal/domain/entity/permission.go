package entity

import (
	"time"

	"github.com/motixo/goat-api/internal/domain/valueobject"
)

type Permission struct {
	ID        string
	Role      valueobject.UserRole
	Action    valueobject.Permission
	CreatedAt time.Time
}
