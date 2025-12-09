package user

import (
	"time"

	"github.com/motixo/goat-api/internal/domain/valueobject"
)

type UserResponse struct {
	ID        string    `json:"id"`
	Email     string    `json:"email"`
	Role      string    `json:"Role"`
	Status    string    `json:"Status"`
	CreatedAt time.Time `json:"createdAt"`
}

type UserUpdateInput struct {
	UserID string
	Email  *string
	Status *valueobject.UserStatus
	Role   *valueobject.UserRole
}

type UpdatePassInput struct {
	UserID      string
	OldPassword string `json:"current_password" binding:"required"`
	NewPassword string `json:"new_password" binding:"required"`
}

type UpdateRoleInput struct {
	UserID string               `json:"user_id" binding:"required"`
	Role   valueobject.UserRole `json:"role" binding:"required"`
}

type UpdateStatusInput struct {
	UserID string                 `json:"user_id" binding:"required"`
	Status valueobject.UserStatus `json:"status" binding:"required"`
}
