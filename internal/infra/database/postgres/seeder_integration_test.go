package postgres

import (
	"context"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/jmoiron/sqlx"
	"github.com/motixo/goat-api/internal/domain/valueobject"
)

func TestSeedPermissionsIntegrationIsIdempotent(t *testing.T) {
	db := newPostgresSeedIntegrationDB(t, false)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	if err := SeedPermissions(ctx, db); err != nil {
		t.Fatalf("SeedPermissions(first) error = %v", err)
	}
	if err := SeedPermissions(ctx, db); err != nil {
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

	if err := SeedPermissions(context.Background(), db); err == nil {
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

func TestSeedPermissionsIntegrationRollsBackOnTimeout(t *testing.T) {
	db, inspector, schemaIdentifier := newPostgresSeedTimeoutIntegrationDB(t)

	const advisoryLockKey int64 = 715603113
	var acquired bool
	if err := inspector.Get(
		&acquired,
		`SELECT pg_try_advisory_lock($1)`,
		advisoryLockKey,
	); err != nil {
		t.Fatalf("acquire advisory lock: %v", err)
	}
	if !acquired {
		t.Fatal("advisory lock was already held")
	}
	t.Cleanup(func() {
		_, _ = inspector.Exec(`SELECT pg_advisory_unlock($1)`, advisoryLockKey)
	})

	if _, err := db.Exec(`
		CREATE OR REPLACE FUNCTION ` + schemaIdentifier + `.block_permission_seed()
		RETURNS TRIGGER
		LANGUAGE plpgsql
		AS $function$
		BEGIN
			PERFORM pg_advisory_xact_lock(715603113);
			RETURN NEW;
		END;
		$function$
	`); err != nil {
		t.Fatalf("create blocking seed function: %v", err)
	}
	if _, err := db.Exec(`
		CREATE TRIGGER block_permission_seed
		BEFORE INSERT ON permissions
		FOR EACH ROW EXECUTE FUNCTION ` + schemaIdentifier + `.block_permission_seed()
	`); err != nil {
		t.Fatalf("create blocking seed trigger: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	err := SeedPermissions(ctx, db)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("SeedPermissions() error = %v, want context.DeadlineExceeded", err)
	}

	if _, err := inspector.Exec(`SELECT pg_advisory_unlock($1)`, advisoryLockKey); err != nil {
		t.Fatalf("release advisory lock: %v", err)
	}
	var count int
	if err := inspector.Get(
		&count,
		`SELECT COUNT(*) FROM `+schemaIdentifier+`.permissions`,
	); err != nil {
		t.Fatalf("count permissions after timed-out seed: %v", err)
	}
	if count != 0 {
		t.Fatalf("permission count after timed-out seed = %d, want 0 after rollback", count)
	}
}

func newPostgresSeedTimeoutIntegrationDB(
	t *testing.T,
) (*sqlx.DB, *sqlx.DB, string) {
	t.Helper()
	dsn := os.Getenv("GOAT_POSTGRES_TEST_DSN")
	if dsn == "" {
		t.Skip("set GOAT_POSTGRES_TEST_DSN to run PostgreSQL integration tests")
	}

	inspector, err := sqlx.Connect("pgx", dsn)
	if err != nil {
		t.Fatalf("connect PostgreSQL inspector: %v", err)
	}
	inspector.SetMaxOpenConns(1)
	inspector.SetMaxIdleConns(1)

	schemaName := fmt.Sprintf("goat_seed_timeout_%d_%d", os.Getpid(), time.Now().UnixNano())
	schemaIdentifier := pgx.Identifier{schemaName}.Sanitize()
	if _, err := inspector.Exec(`CREATE SCHEMA ` + schemaIdentifier); err != nil {
		_ = inspector.Close()
		t.Fatalf("create seed timeout schema: %v", err)
	}
	if _, err := inspector.Exec(`
		CREATE TABLE ` + schemaIdentifier + `.permissions (
			id UUID PRIMARY KEY,
			role SMALLINT NOT NULL,
			action TEXT NOT NULL,
			created_at TIMESTAMP NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMP NULL,
			CONSTRAINT unique_role_action UNIQUE(role, action)
		)
	`); err != nil {
		_, _ = inspector.Exec(`DROP SCHEMA ` + schemaIdentifier + ` CASCADE`)
		_ = inspector.Close()
		t.Fatalf("create seed timeout permissions table: %v", err)
	}

	pgxConfig, err := pgx.ParseConfig(dsn)
	if err != nil {
		_, _ = inspector.Exec(`DROP SCHEMA ` + schemaIdentifier + ` CASCADE`)
		_ = inspector.Close()
		t.Fatalf("parse PostgreSQL test DSN: %v", err)
	}
	pgxConfig.RuntimeParams["search_path"] = schemaName
	seedDatabase := sqlx.NewDb(stdlib.OpenDB(*pgxConfig), "pgx")
	seedDatabase.SetMaxOpenConns(1)
	seedDatabase.SetMaxIdleConns(1)
	if err := seedDatabase.PingContext(context.Background()); err != nil {
		_ = seedDatabase.Close()
		_, _ = inspector.Exec(`DROP SCHEMA ` + schemaIdentifier + ` CASCADE`)
		_ = inspector.Close()
		t.Fatalf("connect PostgreSQL seed database: %v", err)
	}

	t.Cleanup(func() {
		if err := seedDatabase.Close(); err != nil {
			t.Errorf("close PostgreSQL seed database: %v", err)
		}
		if _, err := inspector.Exec(`DROP SCHEMA ` + schemaIdentifier + ` CASCADE`); err != nil {
			t.Errorf("drop seed timeout schema: %v", err)
		}
		if err := inspector.Close(); err != nil {
			t.Errorf("close PostgreSQL inspector: %v", err)
		}
	})
	return seedDatabase, inspector, schemaIdentifier
}

func newPostgresSeedIntegrationDB(t *testing.T, rejectUserUpdate bool) *sqlx.DB {
	t.Helper()
	dsn := os.Getenv("GOAT_POSTGRES_TEST_DSN")
	if dsn == "" {
		t.Skip("set GOAT_POSTGRES_TEST_DSN to run PostgreSQL integration tests")
	}

	db, err := sqlx.Connect("pgx", dsn)
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
