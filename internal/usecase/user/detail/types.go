package detail

import (
	"context"
	"time"

	"github.com/motixo/goat-api/internal/domain/valueobject"
)

// UserDetail is the credential-free application projection used by read-only
// User profile and detail workflows.
type UserDetail struct {
	ID        string
	Email     string
	Role      valueobject.UserRole
	Status    valueobject.UserStatus
	CreatedAt time.Time
}

// UserDetailReader is owned by the read-only User detail application boundary.
type UserDetailReader interface {
	FindDetailByID(ctx context.Context, id string) (UserDetail, error)
}
