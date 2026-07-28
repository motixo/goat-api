package user

import (
	"strings"
	"testing"

	"github.com/motixo/goat-api/internal/domain/valueobject"
)

func TestUserStatusSnapshotQuerySelectsOnlyCommandPreconditions(t *testing.T) {
	const want = `SELECT id, role, status FROM users WHERE id = $1 LIMIT 1`
	if userStatusSnapshotSelectQuery != want {
		t.Fatalf("status snapshot query = %q, want %q", userStatusSnapshotSelectQuery, want)
	}

	lowerQuery := strings.ToLower(userStatusSnapshotSelectQuery)
	for _, forbidden := range []string{
		"password",
		"credential_version",
		"email",
		"created_at",
		"updated_at",
		"select *",
	} {
		if strings.Contains(lowerQuery, forbidden) {
			t.Fatalf(
				"status snapshot query contains forbidden selection %q: %s",
				forbidden,
				userStatusSnapshotSelectQuery,
			)
		}
	}
}

func TestUserStatusSnapshotRowMapsToApplicationSnapshot(t *testing.T) {
	row := userStatusSnapshotRow{
		ID:     "11111111-1111-4111-8111-111111111111",
		Role:   int16(valueobject.RoleOperator),
		Status: int16(valueobject.StatusSuspended),
	}

	got := row.toStatusSnapshot()
	if got.ID != row.ID ||
		got.Role != valueobject.RoleOperator ||
		got.Status != valueobject.StatusSuspended {
		t.Fatalf("mapped status snapshot = %#v, want values from %#v", got, row)
	}
}
