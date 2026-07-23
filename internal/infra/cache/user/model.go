package usercache

import (
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/motixo/goat-api/internal/domain/entity"
	"github.com/motixo/goat-api/internal/domain/valueobject"
)

type userCacheRecord struct {
	ID        string     `json:"id"`
	Role      string     `json:"role"`
	Status    string     `json:"status"`
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt *time.Time `json:"updated_at"`
}

type userAuthorization struct {
	id        string
	role      valueobject.UserRole
	status    valueobject.UserStatus
	createdAt time.Time
	updatedAt *time.Time
}

func userCacheRecordFromDomain(user *entity.User, expectedUserID string) (userCacheRecord, error) {
	authorization, err := userAuthorizationFromDomain(user, expectedUserID)
	if err != nil {
		return userCacheRecord{}, err
	}

	return userCacheRecord{
		ID:        authorization.id,
		Role:      authorization.role.String(),
		Status:    authorization.status.String(),
		CreatedAt: authorization.createdAt,
		UpdatedAt: cloneUserCacheTime(authorization.updatedAt),
	}, nil
}

func userAuthorizationFromDomain(user *entity.User, expectedUserID string) (*userAuthorization, error) {
	if user == nil {
		return nil, fmt.Errorf("user is nil")
	}
	if err := validateUserCacheID(expectedUserID); err != nil {
		return nil, fmt.Errorf("expected user ID: %w", err)
	}
	if err := validateUserCacheID(user.ID); err != nil {
		return nil, err
	}
	if user.ID != expectedUserID {
		return nil, fmt.Errorf("user ID %q does not match cache key identity %q", user.ID, expectedUserID)
	}
	if err := validateUserCacheRole(user.Role); err != nil {
		return nil, err
	}
	if err := validateUserCacheStatus(user.Status); err != nil {
		return nil, err
	}
	if err := validateUserCacheTimestamps(user.CreatedAt, user.UpdatedAt); err != nil {
		return nil, err
	}

	return &userAuthorization{
		id:        user.ID,
		role:      user.Role,
		status:    user.Status,
		createdAt: user.CreatedAt,
		updatedAt: cloneUserCacheTime(user.UpdatedAt),
	}, nil
}

func (record userCacheRecord) toAuthorization(expectedUserID string) (*userAuthorization, error) {
	if err := validateUserCacheID(expectedUserID); err != nil {
		return nil, fmt.Errorf("expected user ID: %w", err)
	}
	if err := validateUserCacheID(record.ID); err != nil {
		return nil, err
	}
	if record.ID != expectedUserID {
		return nil, fmt.Errorf("cached user ID %q does not match cache key identity %q", record.ID, expectedUserID)
	}

	role, err := valueobject.ParseUserRole(record.Role)
	if err != nil {
		return nil, fmt.Errorf("cached user role: %w", err)
	}
	status, err := valueobject.ParseUserStatus(record.Status)
	if err != nil {
		return nil, fmt.Errorf("cached user status: %w", err)
	}
	if err := validateUserCacheTimestamps(record.CreatedAt, record.UpdatedAt); err != nil {
		return nil, err
	}

	return &userAuthorization{
		id:        record.ID,
		role:      role,
		status:    status,
		createdAt: record.CreatedAt,
		updatedAt: cloneUserCacheTime(record.UpdatedAt),
	}, nil
}

func validateUserCacheID(userID string) error {
	if strings.TrimSpace(userID) == "" {
		return fmt.Errorf("user ID is empty")
	}
	if _, err := uuid.Parse(userID); err != nil {
		return fmt.Errorf("invalid user ID %q: %w", userID, err)
	}
	return nil
}

func validateUserCacheRole(role valueobject.UserRole) error {
	parsedRole, err := valueobject.ParseUserRole(role.String())
	if err != nil || parsedRole != role {
		return fmt.Errorf("invalid user cache role: %d", role)
	}
	return nil
}

func validateUserCacheStatus(status valueobject.UserStatus) error {
	parsedStatus, err := valueobject.ParseUserStatus(status.String())
	if err != nil || parsedStatus != status {
		return fmt.Errorf("invalid user cache status: %d", status)
	}
	return nil
}

func validateUserCacheTimestamps(createdAt time.Time, updatedAt *time.Time) error {
	if createdAt.IsZero() {
		return fmt.Errorf("user created_at is zero")
	}
	if updatedAt == nil {
		return nil
	}
	if updatedAt.IsZero() {
		return fmt.Errorf("user updated_at is zero")
	}
	if updatedAt.Before(createdAt) {
		return fmt.Errorf("user updated_at precedes created_at")
	}
	return nil
}

func cloneUserCacheTime(value *time.Time) *time.Time {
	if value == nil {
		return nil
	}
	cloned := *value
	return &cloned
}
