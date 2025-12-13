package user

import (
	"time"

	"github.com/motixo/goat-api/internal/domain/entity"
	"github.com/motixo/goat-api/internal/domain/valueobject"
)

type UserResponse struct {
	ID        string    `json:"id"`
	Email     string    `json:"email"`
	Role      string    `json:"Role"`
	Status    string    `json:"Status"`
	CreatedAt time.Time `json:"createdAt"`
}

type CreateInput struct {
	Email    string                 `json:"email" validate:"required,email"`
	Password string                 `json:"password" binding:"required"`
	Status   valueobject.UserStatus `json:"status" binding:"required"`
	Role     valueobject.UserRole   `json:"role" binding:"required"`
}

type UpdateInput struct {
	UserID   string
	Email    string                 `json:"email" validate:"email"`
	Password string                 `json:"password"`
	Status   valueobject.UserStatus `json:"status"`
	Role     valueobject.UserRole   `json:"role"`
}

type UpdateEmailInput struct {
	UserID string
	Email  string `json:"email" binding:"required"`
}

type UpdatePassInput struct {
	UserID      string
	OldPassword string `json:"current_password" binding:"required"`
	NewPassword string `json:"new_password" binding:"required"`
}

type UpdateRoleInput struct {
	UserID string
	Role   valueobject.UserRole `json:"role" binding:"required"`
}

type UpdateStatusInput struct {
	UserID  string
	ActorID string
	Status  valueobject.UserStatus `json:"status" binding:"required"`
}

type GetListInput struct {
	ActorID string
	Filter  entity.UserFilter
	Offset  int
	Limit   int
}
