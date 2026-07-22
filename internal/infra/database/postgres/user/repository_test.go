package user

import "testing"

func TestBuildUserListSelectQueryUsesStableTieBreaker(t *testing.T) {
	query := buildUserListSelectQuery(" WHERE role = ANY($1)", 2)
	want := "SELECT id, email, role, status, created_at, updated_at FROM users" +
		" WHERE role = ANY($1)" +
		" ORDER BY created_at DESC, id DESC LIMIT $2 OFFSET $3"

	if query != want {
		t.Fatalf("query = %q, want %q", query, want)
	}
}
