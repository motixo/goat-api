package handlers

import (
	"time"

	"github.com/motixo/goat-api/internal/delivery/http/helper"
	"github.com/motixo/goat-api/internal/domain/valueobject"
	"github.com/motixo/goat-api/internal/usecase/user"
)

type listUsersQuery struct {
	helper.PaginationInput
	Filter listUsersFilterQuery
}

type listUsersFilterQuery struct {
	Roles    []string `form:"role"`
	Statuses []string `form:"status"`
	Search   string   `form:"search"`
}

type createUserRequest struct {
	Email    string                 `json:"email" validate:"required,email"`
	Password string                 `json:"password" binding:"required"`
	Status   valueobject.UserStatus `json:"status" binding:"required"`
	Role     valueobject.UserRole   `json:"role" binding:"required"`
}

type updateUserRequest struct {
	Email    string                 `json:"email" validate:"email"`
	Password string                 `json:"password"`
	Status   valueobject.UserStatus `json:"status"`
	Role     valueobject.UserRole   `json:"role"`
}

type updateUserEmailRequest struct {
	Email string `json:"email" binding:"required"`
}

type updateUserPasswordRequest struct {
	CurrentPassword string `json:"current_password" binding:"required"`
	NewPassword     string `json:"new_password" binding:"required"`
}

type updateUserRoleRequest struct {
	Role valueobject.UserRole `json:"role" binding:"required"`
}

type updateUserStatusRequest struct {
	Status valueobject.UserStatus `json:"status" binding:"required"`
}

type userResponse struct {
	ID        string    `json:"id"`
	Email     string    `json:"email"`
	Role      string    `json:"Role"`
	Status    string    `json:"Status"`
	CreatedAt time.Time `json:"createdAt"`
}

func newUserResponse(output user.UserOutput) userResponse {
	return userResponse{
		ID:        output.ID,
		Email:     output.Email,
		Role:      output.Role,
		Status:    output.Status,
		CreatedAt: output.CreatedAt,
	}
}

func newUserResponses(output []user.UserOutput) []userResponse {
	responses := make([]userResponse, 0, len(output))
	for _, item := range output {
		responses = append(responses, newUserResponse(item))
	}
	return responses
}
