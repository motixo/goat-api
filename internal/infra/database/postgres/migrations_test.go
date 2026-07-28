package postgres

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"errors"
	"fmt"
	"slices"
	"testing"
	"testing/fstest"
)

func TestLoadEmbeddedMigrations(t *testing.T) {
	migrations, err := loadEmbeddedMigrations()
	if err != nil {
		t.Fatalf("loadEmbeddedMigrations() error = %v", err)
	}
	want := []struct {
		version int64
		name    string
	}{
		{version: 1, name: "create_users"},
		{version: 2, name: "create_users_created_at_index"},
		{version: 3, name: "create_permissions"},
	}
	if len(migrations) != len(want) {
		t.Fatalf("migration count = %d, want %d", len(migrations), len(want))
	}
	for index, expected := range want {
		got := migrations[index]
		if got.Version != expected.version || got.Name != expected.name {
			t.Fatalf("migration %d = (%d, %q), want (%d, %q)", index, got.Version, got.Name, expected.version, expected.name)
		}
		if got.SQL == "" || len(got.Checksum) != sha256.Size*2 {
			t.Fatalf("migration %06d has empty SQL or invalid checksum", got.Version)
		}
	}
}

func TestLoadMigrationsRejectsInvalidAssets(t *testing.T) {
	tests := []struct {
		name        string
		migrationFS fstest.MapFS
	}{
		{name: "missing migrations", migrationFS: fstest.MapFS{}},
		{
			name: "invalid filename",
			migrationFS: fstest.MapFS{
				"migrations/current.sql": {Data: []byte("SELECT 1")},
			},
		},
		{
			name: "empty migration",
			migrationFS: fstest.MapFS{
				"migrations/000001_empty.sql": {Data: []byte(" \n\t")},
			},
		},
		{
			name: "duplicate version",
			migrationFS: fstest.MapFS{
				"migrations/000001_first.sql":  {Data: []byte("SELECT 1")},
				"migrations/000001_second.sql": {Data: []byte("SELECT 2")},
			},
		},
		{
			name: "sequence gap",
			migrationFS: fstest.MapFS{
				"migrations/000001_first.sql": {Data: []byte("SELECT 1")},
				"migrations/000003_third.sql": {Data: []byte("SELECT 3")},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			migrations, err := loadMigrations(test.migrationFS)
			if !errors.Is(err, ErrMigrationStateInvalid) {
				t.Fatalf("loadMigrations() error = %v, want ErrMigrationStateInvalid", err)
			}
			if migrations != nil {
				t.Fatalf("loadMigrations() = %#v after invalid assets, want nil", migrations)
			}
		})
	}
}

func TestValidateMigrationState(t *testing.T) {
	available := testMigrations(3)
	tests := []struct {
		name        string
		applied     []appliedMigration
		wantPending []migration
		wantErr     error
	}{
		{name: "none applied", wantPending: available},
		{
			name:        "prefix applied",
			applied:     testAppliedMigrations(available[:2]),
			wantPending: available[2:],
		},
		{name: "current", applied: testAppliedMigrations(available)},
		{
			name: "stale version",
			applied: []appliedMigration{
				{Version: 2, Name: available[0].Name, Checksum: available[0].Checksum},
			},
			wantErr: ErrMigrationStateInvalid,
		},
		{
			name: "changed name",
			applied: []appliedMigration{
				{Version: 1, Name: "changed", Checksum: available[0].Checksum},
			},
			wantErr: ErrMigrationStateInvalid,
		},
		{
			name: "changed checksum",
			applied: []appliedMigration{
				{Version: 1, Name: available[0].Name, Checksum: "changed"},
			},
			wantErr: ErrMigrationStateInvalid,
		},
		{
			name:    "unknown applied migration",
			applied: append(testAppliedMigrations(available), appliedMigration{Version: 4}),
			wantErr: ErrMigrationStateInvalid,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			pending, err := validateMigrationState(available, test.applied)
			if !errors.Is(err, test.wantErr) {
				t.Fatalf("validateMigrationState() error = %v, want %v", err, test.wantErr)
			}
			if !slices.Equal(pending, test.wantPending) {
				t.Fatalf("pending migrations = %#v, want %#v", pending, test.wantPending)
			}
		})
	}
}

func TestValidateCurrentMigrationsRequiresMetadata(t *testing.T) {
	reader := migrationReaderFake{
		get: func(_ context.Context, destination any, _ string, _ ...any) error {
			relation := destination.(*sql.NullString)
			*relation = sql.NullString{}
			return nil
		},
	}

	err := validateCurrentMigrations(context.Background(), reader, testMigrations(1))
	if !errors.Is(err, ErrMigrationsPending) {
		t.Fatalf("validateCurrentMigrations() error = %v, want ErrMigrationsPending", err)
	}
}

func TestValidateCurrentMigrationsPreservesCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	reader := migrationReaderFake{
		get: func(ctx context.Context, _ any, _ string, _ ...any) error {
			return ctx.Err()
		},
	}

	err := validateCurrentMigrations(ctx, reader, testMigrations(1))
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("validateCurrentMigrations() error = %v, want context.Canceled", err)
	}
}

func testMigrations(count int) []migration {
	migrations := make([]migration, count)
	for index := range count {
		version := int64(index + 1)
		migrations[index] = migration{
			Version:  version,
			Name:     fmt.Sprintf("migration_%d", version),
			SQL:      fmt.Sprintf("SELECT %d", version),
			Checksum: fmt.Sprintf("checksum_%d", version),
		}
	}
	return migrations
}

func testAppliedMigrations(migrations []migration) []appliedMigration {
	applied := make([]appliedMigration, len(migrations))
	for index, migration := range migrations {
		applied[index] = appliedMigration{
			Version:  migration.Version,
			Name:     migration.Name,
			Checksum: migration.Checksum,
		}
	}
	return applied
}

type migrationReaderFake struct {
	get        func(context.Context, any, string, ...any) error
	selectRows func(context.Context, any, string, ...any) error
}

func (f migrationReaderFake) GetContext(
	ctx context.Context,
	destination any,
	query string,
	args ...any,
) error {
	return f.get(ctx, destination, query, args...)
}

func (f migrationReaderFake) SelectContext(
	ctx context.Context,
	destination any,
	query string,
	args ...any,
) error {
	if f.selectRows == nil {
		return errors.New("unexpected SelectContext call")
	}
	return f.selectRows(ctx, destination, query, args...)
}
