package user

import (
	"reflect"
	"testing"
	"time"

	"github.com/motixo/goat-api/internal/domain/valueobject"
	userusecase "github.com/motixo/goat-api/internal/usecase/user"
)

func TestUserListRowMapsToCredentialFreeApplicationProjection(t *testing.T) {
	createdAt := time.Date(2026, time.July, 27, 9, 30, 0, 0, time.UTC)
	row := userListRow{
		ID:        "33333333-3333-4333-8333-333333333333",
		Email:     "listed@example.com",
		Status:    int16(valueobject.StatusActive),
		Role:      int16(valueobject.RoleClient),
		CreatedAt: createdAt,
	}

	got := row.toListItem()
	want := userusecase.UserListItem{
		ID:        row.ID,
		Email:     row.Email,
		Status:    valueobject.StatusActive,
		Role:      valueobject.RoleClient,
		CreatedAt: createdAt,
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("userListRow.toListItem() = %#v, want %#v", got, want)
	}

	projectionType := reflect.TypeOf(got)
	for _, forbidden := range []string{"Password", "PasswordDigest", "CredentialVersion"} {
		if _, exists := projectionType.FieldByName(forbidden); exists {
			t.Fatalf("UserListItem contains forbidden credential field %q", forbidden)
		}
	}
}
