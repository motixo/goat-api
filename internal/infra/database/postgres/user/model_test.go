package user

import (
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/motixo/goat-api/internal/domain/entity"
	domainErrors "github.com/motixo/goat-api/internal/domain/errors"
	"github.com/motixo/goat-api/internal/domain/valueobject"
)

func TestUserRowToDomainPreservesAllFields(t *testing.T) {
	createdAt := time.Date(2026, time.July, 23, 8, 30, 0, 0, time.UTC)
	updatedAt := time.Date(2026, time.July, 23, 9, 45, 0, 0, time.UTC)
	row := userRow{
		ID:                "11111111-1111-4111-8111-111111111111",
		Email:             "user@example.com",
		PasswordHash:      "$argon2id$mapped-hash",
		Status:            int16(valueobject.StatusSuspended),
		Role:              int16(valueobject.RoleOperator),
		CredentialVersion: 7,
		CreatedAt:         createdAt,
		UpdatedAt:         &updatedAt,
	}

	got, err := row.toDomain()
	if err != nil {
		t.Fatalf("userRow.toDomain() error = %v", err)
	}
	want := &entity.User{
		ID:                row.ID,
		Email:             row.Email,
		PasswordDigest:    testPasswordDigest(row.PasswordHash),
		Status:            valueobject.StatusSuspended,
		Role:              valueobject.RoleOperator,
		CredentialVersion: 7,
		CreatedAt:         createdAt,
		UpdatedAt:         &updatedAt,
	}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("userRow.toDomain() = %#v, want %#v", got, want)
	}
	if got.PasswordDigest.Encoded() != row.PasswordHash {
		t.Fatalf("mapped password hash = %q, want %q", got.PasswordDigest.Encoded(), row.PasswordHash)
	}
}

func TestUserRowToDomainPreservesNullUpdatedAt(t *testing.T) {
	got, err := (userRow{PasswordHash: "$opaque-digest", UpdatedAt: nil}).toDomain()
	if err != nil {
		t.Fatalf("userRow.toDomain() error = %v", err)
	}
	if got.UpdatedAt != nil {
		t.Fatalf("mapped updated_at = %v, want nil", got.UpdatedAt)
	}
}

func TestUserRowToDomainRejectsEmptyPasswordDigest(t *testing.T) {
	user, err := (userRow{}).toDomain()
	if !errors.Is(err, domainErrors.ErrInvalidStoredPasswordHash) {
		t.Fatalf("userRow.toDomain() error = %v, want ErrInvalidStoredPasswordHash", err)
	}
	if user != nil {
		t.Fatal("userRow.toDomain() returned a user with an empty password digest")
	}
}

func TestUserRowFromDomainPreservesAllFields(t *testing.T) {
	createdAt := time.Date(2026, time.July, 23, 10, 0, 0, 0, time.UTC)
	updatedAt := time.Date(2026, time.July, 23, 10, 30, 0, 0, time.UTC)
	domainUser := &entity.User{
		ID:                "22222222-2222-4222-8222-222222222222",
		Email:             "persist@example.com",
		PasswordDigest:    testPasswordDigest("$argon2id$persistence-hash"),
		Status:            valueobject.StatusActive,
		Role:              valueobject.RoleAdmin,
		CredentialVersion: 11,
		CreatedAt:         createdAt,
		UpdatedAt:         &updatedAt,
	}

	got := userRowFromDomain(domainUser)
	want := userRow{
		ID:                domainUser.ID,
		Email:             domainUser.Email,
		PasswordHash:      domainUser.PasswordDigest.Encoded(),
		Status:            int16(domainUser.Status),
		Role:              int16(domainUser.Role),
		CredentialVersion: domainUser.CredentialVersion,
		CreatedAt:         createdAt,
		UpdatedAt:         &updatedAt,
	}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("userRowFromDomain() = %#v, want %#v", got, want)
	}
}

func TestUserListRowMapsWithoutRehydratingCredentials(t *testing.T) {
	row := userListRow{
		ID:        "33333333-3333-4333-8333-333333333333",
		Email:     "listed@example.com",
		Status:    int16(valueobject.StatusActive),
		Role:      int16(valueobject.RoleClient),
		CreatedAt: time.Date(2026, time.July, 23, 11, 0, 0, 0, time.UTC),
	}

	got := row.toListItem()
	if got.ID != row.ID || got.Email != row.Email ||
		got.Status != valueobject.StatusActive || got.Role != valueobject.RoleClient ||
		!got.CreatedAt.Equal(row.CreatedAt) {
		t.Fatalf("userListRow.toListItem() = %#v, want application list fields", got)
	}
}

func testPasswordDigest(encoded string) valueobject.PasswordDigest {
	digest, err := valueobject.NewPasswordDigest(encoded)
	if err != nil {
		panic("test password digest is invalid")
	}
	return digest
}
