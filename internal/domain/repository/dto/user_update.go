package dto

import "github.com/motixo/goat-api/internal/domain/valueobject"

type UserUpdate struct {
	Email    *string
	Password *valueobject.Password
	Status   *valueobject.UserStatus
	Role     *valueobject.UserRole
}
