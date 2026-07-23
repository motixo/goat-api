package user

import (
	"time"

	"github.com/motixo/goat-api/internal/domain/valueobject"
)

type UserOutput struct {
	ID        string
	Email     string
	Role      string
	Status    string
	CreatedAt time.Time
}

type CreateInput struct {
	Email    string
	Password string
	Status   valueobject.UserStatus
	Role     valueobject.UserRole
}

type UpdateInput struct {
	UserID   string
	Email    string
	Password string
	Status   valueobject.UserStatus
	Role     valueobject.UserRole
}

type UpdateEmailInput struct {
	UserID string
	Email  string
}

type UpdatePassInput struct {
	UserID      string
	OldPassword string
	NewPassword string
}

type UpdateRoleInput struct {
	UserID string
	Role   valueobject.UserRole
}

type UpdateStatusInput struct {
	UserID  string
	ActorID string
	Status  valueobject.UserStatus
}

type ListFilter struct {
	Statuses []valueobject.UserStatus
	Roles    []valueobject.UserRole
	Search   string
	// MatchNone preserves the meaning of a supplied filter containing no known values.
	MatchNone bool
}

type GetListInput struct {
	ActorID string
	Filter  ListFilter
	Offset  int
	Limit   int
}
