package user

import (
	"reflect"
	"testing"
	"time"

	"github.com/motixo/goat-api/internal/domain/entity"
	"github.com/motixo/goat-api/internal/domain/valueobject"
)

func TestUserRowToDomainPreservesAllFields(t *testing.T) {
	createdAt := time.Date(2026, time.July, 23, 8, 30, 0, 0, time.UTC)
	updatedAt := time.Date(2026, time.July, 23, 9, 45, 0, 0, time.UTC)
	row := userRow{
		ID:           "11111111-1111-4111-8111-111111111111",
		Email:        "user@example.com",
		PasswordHash: "$argon2id$mapped-hash",
		Status:       int16(valueobject.StatusSuspended),
		Role:         int16(valueobject.RoleOperator),
		CreatedAt:    createdAt,
		UpdatedAt:    &updatedAt,
	}

	got := row.toDomain()
	want := &entity.User{
		ID:        row.ID,
		Email:     row.Email,
		Password:  valueobject.PasswordFromHash(row.PasswordHash),
		Status:    valueobject.StatusSuspended,
		Role:      valueobject.RoleOperator,
		CreatedAt: createdAt,
		UpdatedAt: &updatedAt,
	}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("userRow.toDomain() = %#v, want %#v", got, want)
	}
	if got.Password.Encoded() != row.PasswordHash {
		t.Fatalf("mapped password hash = %q, want %q", got.Password.Encoded(), row.PasswordHash)
	}
}

func TestUserRowToDomainPreservesNullUpdatedAt(t *testing.T) {
	got := (userRow{UpdatedAt: nil}).toDomain()
	if got.UpdatedAt != nil {
		t.Fatalf("mapped updated_at = %v, want nil", got.UpdatedAt)
	}
}

func TestUserRowFromDomainPreservesAllFields(t *testing.T) {
	createdAt := time.Date(2026, time.July, 23, 10, 0, 0, 0, time.UTC)
	updatedAt := time.Date(2026, time.July, 23, 10, 30, 0, 0, time.UTC)
	domainUser := &entity.User{
		ID:        "22222222-2222-4222-8222-222222222222",
		Email:     "persist@example.com",
		Password:  valueobject.PasswordFromHash("$argon2id$persistence-hash"),
		Status:    valueobject.StatusActive,
		Role:      valueobject.RoleAdmin,
		CreatedAt: createdAt,
		UpdatedAt: &updatedAt,
	}

	got := userRowFromDomain(domainUser)
	want := userRow{
		ID:           domainUser.ID,
		Email:        domainUser.Email,
		PasswordHash: domainUser.Password.Encoded(),
		Status:       int16(domainUser.Status),
		Role:         int16(domainUser.Role),
		CreatedAt:    createdAt,
		UpdatedAt:    &updatedAt,
	}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("userRowFromDomain() = %#v, want %#v", got, want)
	}
}
