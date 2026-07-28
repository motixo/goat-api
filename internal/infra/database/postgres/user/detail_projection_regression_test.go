package user

import (
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/motixo/goat-api/internal/domain/valueobject"
	userdetail "github.com/motixo/goat-api/internal/usecase/user/detail"
)

func TestUserDetailRowMapsToCredentialFreeApplicationProjection(t *testing.T) {
	createdAt := time.Date(2026, time.July, 27, 12, 45, 0, 0, time.UTC)
	row := userDetailRow{
		ID:        "22222222-2222-4222-8222-222222222222",
		Email:     "reader@example.com",
		Status:    int16(valueobject.StatusSuspended),
		Role:      int16(valueobject.RoleAdmin),
		CreatedAt: createdAt,
	}

	got := row.toDetail()
	want := userdetail.UserDetail{
		ID:        row.ID,
		Email:     row.Email,
		Status:    valueobject.StatusSuspended,
		Role:      valueobject.RoleAdmin,
		CreatedAt: createdAt,
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("userDetailRow.toDetail() = %#v, want %#v", got, want)
	}
}

func TestUserDetailQuerySelectsNoCredentialColumns(t *testing.T) {
	want := "SELECT id, email, role, status, created_at FROM users WHERE id = $1 LIMIT 1"
	if userDetailSelectQuery != want {
		t.Fatalf("detail query = %q, want %q", userDetailSelectQuery, want)
	}

	lowerQuery := strings.ToLower(userDetailSelectQuery)
	for _, forbidden := range []string{"password", "credential_version", "select *"} {
		if strings.Contains(lowerQuery, forbidden) {
			t.Fatalf("detail query contains forbidden selection %q: %s", forbidden, userDetailSelectQuery)
		}
	}
}
