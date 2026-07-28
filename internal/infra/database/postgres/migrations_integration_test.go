package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/jmoiron/sqlx"
)

func TestMigrationsIntegrationApplyValidateAndAreIdempotent(t *testing.T) {
	database := newMigrationIntegrationDatabase(t)
	available := mustLoadEmbeddedMigrations(t)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	first, err := applyMigrations(ctx, database.db, available)
	if err != nil {
		t.Fatalf("applyMigrations(first) error = %v", err)
	}
	if first.Applied != len(available) || first.CurrentVersion != available[len(available)-1].Version {
		t.Fatalf("first migration result = %#v, want %d applied at version %d", first, len(available), available[len(available)-1].Version)
	}
	if err := validateCurrentMigrations(ctx, database.db, available); err != nil {
		t.Fatalf("validateCurrentMigrations() error = %v", err)
	}

	second, err := applyMigrations(ctx, database.db, available)
	if err != nil {
		t.Fatalf("applyMigrations(second) error = %v", err)
	}
	if second.Applied != 0 || second.CurrentVersion != first.CurrentVersion {
		t.Fatalf("second migration result = %#v, want idempotent version %d", second, first.CurrentVersion)
	}

	var recorded int
	if err := database.db.GetContext(ctx, &recorded, `SELECT COUNT(*) FROM schema_migrations`); err != nil {
		t.Fatalf("count migration metadata: %v", err)
	}
	if recorded != len(available) {
		t.Fatalf("recorded migration count = %d, want %d", recorded, len(available))
	}

	var userColumns []string
	if err := database.db.SelectContext(ctx, &userColumns, `
		SELECT column_name
		FROM information_schema.columns
		WHERE table_schema = current_schema() AND table_name = 'users'
		ORDER BY ordinal_position
	`); err != nil {
		t.Fatalf("read migrated user columns: %v", err)
	}
	wantUserColumns := []string{
		"id",
		"email",
		"password",
		"status",
		"role",
		"credential_version",
		"created_at",
		"updated_at",
	}
	if !slices.Equal(userColumns, wantUserColumns) {
		t.Fatalf("migrated user columns = %v, want %v", userColumns, wantUserColumns)
	}

	var indexDefinition string
	if err := database.db.GetContext(ctx, &indexDefinition, `
		SELECT indexdef
		FROM pg_indexes
		WHERE schemaname = current_schema()
		  AND indexname = 'idx_users_created_at_desc'
	`); err != nil {
		t.Fatalf("read migrated user index: %v", err)
	}
	if !strings.Contains(indexDefinition, "(created_at DESC)") {
		t.Fatalf("migrated user index = %q, want deterministic descending created_at index", indexDefinition)
	}

	var permissionConstraint bool
	if err := database.db.GetContext(ctx, &permissionConstraint, `
		SELECT EXISTS (
			SELECT 1
			FROM pg_constraint constraint_record
			JOIN pg_class relation ON relation.oid = constraint_record.conrelid
			JOIN pg_namespace namespace ON namespace.oid = relation.relnamespace
			WHERE namespace.nspname = current_schema()
			  AND relation.relname = 'permissions'
			  AND constraint_record.conname = 'unique_role_action'
		)
	`); err != nil {
		t.Fatalf("read migrated permission constraint: %v", err)
	}
	if !permissionConstraint {
		t.Fatal("migrated permissions table is missing unique_role_action")
	}
}

func TestMigrationsIntegrationDetectPendingAndDriftedHistory(t *testing.T) {
	database := newMigrationIntegrationDatabase(t)
	available := mustLoadEmbeddedMigrations(t)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if _, err := applyMigrations(ctx, database.db, available[:1]); err != nil {
		t.Fatalf("applyMigrations(prefix) error = %v", err)
	}
	if err := validateCurrentMigrations(ctx, database.db, available); !errors.Is(err, ErrMigrationsPending) {
		t.Fatalf("validateCurrentMigrations(pending) error = %v, want ErrMigrationsPending", err)
	}

	if _, err := database.db.ExecContext(
		ctx,
		`UPDATE schema_migrations SET checksum = $1 WHERE version = $2`,
		strings.Repeat("0", 64),
		available[0].Version,
	); err != nil {
		t.Fatalf("corrupt migration checksum: %v", err)
	}
	if err := validateCurrentMigrations(ctx, database.db, available); !errors.Is(err, ErrMigrationStateInvalid) {
		t.Fatalf("validateCurrentMigrations(drifted) error = %v, want ErrMigrationStateInvalid", err)
	}
}

func TestMigrationsIntegrationRollBackTheCompleteBatch(t *testing.T) {
	database := newMigrationIntegrationDatabase(t)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	invalidBatch := []migration{
		{Version: 1, Name: "create_probe", SQL: `CREATE TABLE rollback_probe (id BIGINT PRIMARY KEY)`, Checksum: "probe"},
		{Version: 2, Name: "fail_batch", SQL: `CREATE TABL invalid_migration`, Checksum: "invalid"},
	}

	if _, err := applyMigrations(ctx, database.db, invalidBatch); err == nil {
		t.Fatal("applyMigrations() error = nil, want invalid migration failure")
	}
	for _, relationName := range []string{"schema_migrations", "rollback_probe"} {
		var relation sql.NullString
		if err := database.db.GetContext(
			ctx,
			&relation,
			`SELECT to_regclass($1)::text`,
			relationName,
		); err != nil {
			t.Fatalf("locate %s after rollback: %v", relationName, err)
		}
		if relation.Valid {
			t.Fatalf("relation %s survived failed migration transaction", relationName)
		}
	}
}

func TestMigrationsIntegrationSerializesConcurrentRuns(t *testing.T) {
	database := newMigrationIntegrationDatabase(t)
	available := mustLoadEmbeddedMigrations(t)
	start := make(chan struct{})
	results := make(chan struct {
		result MigrationResult
		err    error
	}, 2)

	var ready sync.WaitGroup
	ready.Add(2)
	for range 2 {
		go func() {
			ready.Done()
			<-start
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			result, err := applyMigrations(ctx, database.db, available)
			results <- struct {
				result MigrationResult
				err    error
			}{result: result, err: err}
		}()
	}
	ready.Wait()
	close(start)

	totalApplied := 0
	for range 2 {
		outcome := <-results
		if outcome.err != nil {
			t.Fatalf("concurrent applyMigrations() error = %v", outcome.err)
		}
		if outcome.result.CurrentVersion != available[len(available)-1].Version {
			t.Fatalf("concurrent migration version = %d, want %d", outcome.result.CurrentVersion, available[len(available)-1].Version)
		}
		totalApplied += outcome.result.Applied
	}
	if totalApplied != len(available) {
		t.Fatalf("concurrent migration applied count = %d, want exactly %d", totalApplied, len(available))
	}
}

func TestMigrationsIntegrationLockWaitHonorsContext(t *testing.T) {
	database := newMigrationIntegrationDatabase(t)
	available := mustLoadEmbeddedMigrations(t)

	if _, err := database.inspector.Exec(
		`SELECT pg_advisory_lock($1)`,
		migrationAdvisoryLockKey,
	); err != nil {
		t.Fatalf("acquire competing migration lock: %v", err)
	}
	lockHeld := true
	t.Cleanup(func() {
		if lockHeld {
			_, _ = database.inspector.Exec(`SELECT pg_advisory_unlock($1)`, migrationAdvisoryLockKey)
		}
	})

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	if _, err := applyMigrations(ctx, database.db, available); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("applyMigrations(blocked) error = %v, want context.DeadlineExceeded", err)
	}
	if _, err := database.inspector.Exec(
		`SELECT pg_advisory_unlock($1)`,
		migrationAdvisoryLockKey,
	); err != nil {
		t.Fatalf("release competing migration lock: %v", err)
	}
	lockHeld = false

	var relation sql.NullString
	if err := database.db.Get(
		&relation,
		`SELECT to_regclass('schema_migrations')::text`,
	); err != nil {
		t.Fatalf("locate migration metadata after timeout: %v", err)
	}
	if relation.Valid {
		t.Fatal("migration metadata was partially created after lock timeout")
	}
}

type migrationIntegrationDatabase struct {
	db        *sqlx.DB
	inspector *sqlx.DB
}

func newMigrationIntegrationDatabase(t *testing.T) migrationIntegrationDatabase {
	t.Helper()
	dsn := os.Getenv("GOAT_POSTGRES_TEST_DSN")
	if dsn == "" {
		t.Skip("set GOAT_POSTGRES_TEST_DSN to run PostgreSQL integration tests")
	}

	inspector, err := sqlx.Connect("pgx", dsn)
	if err != nil {
		t.Fatalf("connect PostgreSQL migration inspector: %v", err)
	}
	inspector.SetMaxOpenConns(1)
	inspector.SetMaxIdleConns(1)

	schemaName := fmt.Sprintf("goat_migration_%d_%d", os.Getpid(), time.Now().UnixNano())
	schemaIdentifier := pgx.Identifier{schemaName}.Sanitize()
	if _, err := inspector.Exec(`CREATE SCHEMA ` + schemaIdentifier); err != nil {
		_ = inspector.Close()
		t.Fatalf("create migration integration schema: %v", err)
	}

	connectionConfig, err := pgx.ParseConfig(dsn)
	if err != nil {
		_, _ = inspector.Exec(`DROP SCHEMA ` + schemaIdentifier + ` CASCADE`)
		_ = inspector.Close()
		t.Fatalf("parse PostgreSQL migration test DSN: %v", err)
	}
	connectionConfig.RuntimeParams["search_path"] = schemaName
	database := sqlx.NewDb(stdlib.OpenDB(*connectionConfig), driverName)
	database.SetMaxOpenConns(4)
	database.SetMaxIdleConns(4)
	if err := database.PingContext(context.Background()); err != nil {
		_ = database.Close()
		_, _ = inspector.Exec(`DROP SCHEMA ` + schemaIdentifier + ` CASCADE`)
		_ = inspector.Close()
		t.Fatalf("connect PostgreSQL migration database: %v", err)
	}

	t.Cleanup(func() {
		if err := database.Close(); err != nil {
			t.Errorf("close PostgreSQL migration database: %v", err)
		}
		if _, err := inspector.Exec(`DROP SCHEMA ` + schemaIdentifier + ` CASCADE`); err != nil {
			t.Errorf("drop PostgreSQL migration schema: %v", err)
		}
		if err := inspector.Close(); err != nil {
			t.Errorf("close PostgreSQL migration inspector: %v", err)
		}
	})
	return migrationIntegrationDatabase{
		db:        database,
		inspector: inspector,
	}
}

func mustLoadEmbeddedMigrations(t *testing.T) []migration {
	t.Helper()
	migrations, err := loadEmbeddedMigrations()
	if err != nil {
		t.Fatalf("loadEmbeddedMigrations() error = %v", err)
	}
	return migrations
}
