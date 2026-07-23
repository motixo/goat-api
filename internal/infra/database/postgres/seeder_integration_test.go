package postgres

import (
	"os"
	"testing"
	"time"

	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
	"github.com/motixo/goat-api/internal/domain/valueobject"
)

func TestSeedPermissionsIntegrationIsIdempotent(t *testing.T) {
	db := newPostgresSeedIntegrationDB(t, false)

	if err := SeedPermissions(db); err != nil {
		t.Fatalf("SeedPermissions(first) error = %v", err)
	}
	if err := SeedPermissions(db); err != nil {
		t.Fatalf("SeedPermissions(second) error = %v", err)
	}

	type seededPermission struct {
		Role      int16     `db:"role"`
		Action    string    `db:"action"`
		CreatedAt time.Time `db:"created_at"`
	}
	var got []seededPermission
	if err := db.Select(&got, `SELECT role, action, created_at FROM permissions ORDER BY role DESC, action ASC`); err != nil {
		t.Fatalf("select seeded permissions: %v", err)
	}
	want := []seededPermission{
		{Role: int16(valueobject.RoleAdmin), Action: valueobject.PermFullAccess.String()},
		{Role: int16(valueobject.RoleOperator), Action: valueobject.PermUserChangeStatus.String()},
		{Role: int16(valueobject.RoleOperator), Action: valueobject.PermUserRead.String()},
		{Role: int16(valueobject.RoleOperator), Action: valueobject.PermUserUpdate.String()},
	}
	if len(got) != len(want) {
		t.Fatalf("seeded permission count = %d, want %d", len(got), len(want))
	}
	for index := range want {
		if got[index].Role != want[index].Role || got[index].Action != want[index].Action {
			t.Fatalf("seeded permission %d = %#v, want role %d action %q", index, got[index], want[index].Role, want[index].Action)
		}
		if got[index].CreatedAt.IsZero() {
			t.Fatalf("seeded permission %d has zero created_at", index)
		}
	}
}

func TestSeedPermissionsIntegrationRollsBackOnFailure(t *testing.T) {
	db := newPostgresSeedIntegrationDB(t, true)

	if err := SeedPermissions(db); err == nil {
		t.Fatal("SeedPermissions() error = nil, want rejected permission error")
	}

	var count int
	if err := db.Get(&count, `SELECT COUNT(*) FROM permissions`); err != nil {
		t.Fatalf("count permissions after failed seed: %v", err)
	}
	if count != 0 {
		t.Fatalf("permission count after failed seed = %d, want 0 after rollback", count)
	}
}

func newPostgresSeedIntegrationDB(t *testing.T, rejectUserUpdate bool) *sqlx.DB {
	t.Helper()
	dsn := os.Getenv("GOAT_POSTGRES_TEST_DSN")
	if dsn == "" {
		t.Skip("set GOAT_POSTGRES_TEST_DSN to run PostgreSQL integration tests")
	}

	db, err := sqlx.Connect("postgres", dsn)
	if err != nil {
		t.Fatalf("connect to PostgreSQL: %v", err)
	}
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	t.Cleanup(func() {
		if err := db.Close(); err != nil {
			t.Errorf("close PostgreSQL: %v", err)
		}
	})

	checkConstraint := ""
	if rejectUserUpdate {
		checkConstraint = `, CONSTRAINT reject_user_update CHECK (action <> 'user:update')`
	}
	schema := `
		CREATE TEMP TABLE permissions (
			id UUID PRIMARY KEY,
			role SMALLINT NOT NULL,
			action TEXT NOT NULL,
			created_at TIMESTAMP NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMP NULL,
			CONSTRAINT unique_role_action UNIQUE(role, action)` + checkConstraint + `
		) ON COMMIT PRESERVE ROWS
	`
	if _, err := db.Exec(schema); err != nil {
		t.Fatalf("create temporary permissions table: %v", err)
	}
	return db
}
