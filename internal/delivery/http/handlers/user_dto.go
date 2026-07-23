package handlers

import (
	"encoding/json"
	"fmt"
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
	Email    string          `json:"email" validate:"required,email"`
	Password string          `json:"password" binding:"required"`
	Status   json.RawMessage `json:"status" binding:"required"`
	Role     json.RawMessage `json:"role" binding:"required"`
}

type updateUserRequest struct {
	Email    string          `json:"email" validate:"email"`
	Password string          `json:"password"`
	Status   json.RawMessage `json:"status"`
	Role     json.RawMessage `json:"role"`
}

type updateUserEmailRequest struct {
	Email string `json:"email" binding:"required"`
}

type updateUserPasswordRequest struct {
	CurrentPassword string `json:"current_password" binding:"required"`
	NewPassword     string `json:"new_password" binding:"required"`
}

type updateUserRoleRequest struct {
	Role json.RawMessage `json:"role" binding:"required"`
}

type updateUserStatusRequest struct {
	Status json.RawMessage `json:"status" binding:"required"`
}

type userResponse struct {
	ID        string    `json:"id"`
	Email     string    `json:"email"`
	Role      string    `json:"Role"`
	Status    string    `json:"Status"`
	CreatedAt time.Time `json:"createdAt"`
}

func (query listUsersQuery) toInput(actorID string) user.GetListInput {
	filter := user.ListFilter{Search: query.Filter.Search}
	for _, rawRole := range query.Filter.Roles {
		role, err := valueobject.ParseUserRole(rawRole)
		if err == nil {
			filter.Roles = append(filter.Roles, role)
		}
	}
	if len(query.Filter.Roles) != 0 && len(filter.Roles) == 0 {
		filter.MatchNone = true
	}

	for _, rawStatus := range query.Filter.Statuses {
		status, err := valueobject.ParseUserStatus(rawStatus)
		if err == nil {
			filter.Statuses = append(filter.Statuses, status)
		}
	}
	if len(query.Filter.Statuses) != 0 && len(filter.Statuses) == 0 {
		filter.MatchNone = true
	}

	return user.GetListInput{
		ActorID: actorID,
		Filter:  filter,
		Offset:  query.Offset(),
		Limit:   query.Limit,
	}
}

func (request createUserRequest) toInput() (user.CreateInput, error) {
	status, err := parseUserStatusRequest(request.Status)
	if err != nil {
		return user.CreateInput{}, err
	}
	role, err := parseUserRoleRequest(request.Role)
	if err != nil {
		return user.CreateInput{}, err
	}

	return user.CreateInput{
		Email:    request.Email,
		Password: request.Password,
		Status:   status,
		Role:     role,
	}, nil
}

func (request updateUserRequest) toInput(userID string) (user.UpdateInput, error) {
	status := valueobject.StatusUnknown
	if len(request.Status) != 0 {
		parsedStatus, err := parseUserStatusRequest(request.Status)
		if err != nil {
			return user.UpdateInput{}, err
		}
		status = parsedStatus
	}

	role := valueobject.RoleUnknown
	if len(request.Role) != 0 {
		parsedRole, err := parseUserRoleRequest(request.Role)
		if err != nil {
			return user.UpdateInput{}, err
		}
		role = parsedRole
	}

	return user.UpdateInput{
		UserID:   userID,
		Email:    request.Email,
		Password: request.Password,
		Status:   status,
		Role:     role,
	}, nil
}

func (request updateUserRoleRequest) toInput(userID string) (user.UpdateRoleInput, error) {
	role, err := parseUserRoleRequest(request.Role)
	if err != nil {
		return user.UpdateRoleInput{}, err
	}
	return user.UpdateRoleInput{UserID: userID, Role: role}, nil
}

func (request updateUserStatusRequest) toInput(userID, actorID string) (user.UpdateStatusInput, error) {
	status, err := parseUserStatusRequest(request.Status)
	if err != nil {
		return user.UpdateStatusInput{}, err
	}
	return user.UpdateStatusInput{UserID: userID, ActorID: actorID, Status: status}, nil
}

func parseUserRoleRequest(raw json.RawMessage) (valueobject.UserRole, error) {
	if len(raw) == 0 {
		return valueobject.RoleUnknown, fmt.Errorf("user role is required")
	}

	var value string
	if err := json.Unmarshal(raw, &value); err != nil {
		return valueobject.RoleUnknown, fmt.Errorf("user role must be a string: %w", err)
	}
	return valueobject.ParseUserRole(value)
}

func parseUserStatusRequest(raw json.RawMessage) (valueobject.UserStatus, error) {
	if len(raw) == 0 {
		return valueobject.StatusUnknown, fmt.Errorf("user status is required")
	}

	var value string
	if err := json.Unmarshal(raw, &value); err != nil {
		return valueobject.StatusUnknown, fmt.Errorf("user status must be a string: %w", err)
	}
	return valueobject.ParseUserStatus(value)
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
