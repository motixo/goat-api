package permission

import (
	"time"

	"github.com/motixo/goat-api/internal/domain/valueobject"
)

type CreateInput struct {
	Role   valueobject.UserRole
	Action valueobject.Permission
}

type PermissionOutput struct {
	ID        string
	Role      string
	Action    string
	CreatedAt time.Time
}
