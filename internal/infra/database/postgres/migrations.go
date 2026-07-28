package postgres

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/jmoiron/sqlx"
)

var (
	// ErrMigrationsPending means the database schema is behind the embedded
	// migration set. Application startup must fail until the deployment runs
	// the migration command.
	ErrMigrationsPending = errors.New("PostgreSQL schema migrations are pending")
	// ErrMigrationStateInvalid means applied migration history no longer
	// matches the immutable embedded migration set.
	ErrMigrationStateInvalid = errors.New("PostgreSQL schema migration state is invalid")
)

const (
	migrationAdvisoryLockKey = int64(0x474F41544D494752) // "GOATMIGR"
)

var migrationFilenamePattern = regexp.MustCompile(
	`^migrations/([0-9]{6})_([a-z][a-z0-9_]*)\.sql$`,
)

//go:embed migrations/*.sql
var embeddedMigrationFiles embed.FS

type migration struct {
	Version  int64
	Name     string
	SQL      string
	Checksum string
}

type appliedMigration struct {
	Version  int64  `db:"version"`
	Name     string `db:"name"`
	Checksum string `db:"checksum"`
}

type migrationStateReader interface {
	GetContext(context.Context, any, string, ...any) error
	SelectContext(context.Context, any, string, ...any) error
}

func loadEmbeddedMigrations() ([]migration, error) {
	return loadMigrations(embeddedMigrationFiles)
}

func loadMigrations(migrationFS fs.FS) ([]migration, error) {
	paths, err := fs.Glob(migrationFS, "migrations/*.sql")
	if err != nil {
		return nil, fmt.Errorf("enumerate embedded PostgreSQL migrations: %w", err)
	}
	if len(paths) == 0 {
		return nil, fmt.Errorf("%w: no embedded migrations", ErrMigrationStateInvalid)
	}
	sort.Strings(paths)

	migrations := make([]migration, 0, len(paths))
	seenVersions := make(map[int64]struct{}, len(paths))
	for index, path := range paths {
		matches := migrationFilenamePattern.FindStringSubmatch(path)
		if matches == nil {
			return nil, fmt.Errorf(
				"%w: invalid migration filename %q",
				ErrMigrationStateInvalid,
				path,
			)
		}
		version, parseErr := strconv.ParseInt(matches[1], 10, 64)
		if parseErr != nil || version <= 0 {
			return nil, fmt.Errorf(
				"%w: invalid migration version in %q",
				ErrMigrationStateInvalid,
				path,
			)
		}
		if _, duplicate := seenVersions[version]; duplicate {
			return nil, fmt.Errorf(
				"%w: duplicate migration version %06d",
				ErrMigrationStateInvalid,
				version,
			)
		}
		seenVersions[version] = struct{}{}
		if wantVersion := int64(index + 1); version != wantVersion {
			return nil, fmt.Errorf(
				"%w: migration sequence expected %06d but found %06d",
				ErrMigrationStateInvalid,
				wantVersion,
				version,
			)
		}

		contents, readErr := fs.ReadFile(migrationFS, path)
		if readErr != nil {
			return nil, fmt.Errorf("read embedded PostgreSQL migration %06d: %w", version, readErr)
		}
		if strings.TrimSpace(string(contents)) == "" {
			return nil, fmt.Errorf(
				"%w: migration %06d is empty",
				ErrMigrationStateInvalid,
				version,
			)
		}
		checksum := sha256.Sum256(contents)
		migrations = append(migrations, migration{
			Version:  version,
			Name:     matches[2],
			SQL:      string(contents),
			Checksum: fmt.Sprintf("%x", checksum),
		})
	}

	return migrations, nil
}

func validateMigrationState(
	available []migration,
	applied []appliedMigration,
) ([]migration, error) {
	if len(available) == 0 {
		return nil, fmt.Errorf("%w: no migrations are available", ErrMigrationStateInvalid)
	}
	if len(applied) > len(available) {
		return nil, fmt.Errorf(
			"%w: database has %d migration(s), binary knows %d",
			ErrMigrationStateInvalid,
			len(applied),
			len(available),
		)
	}
	for index, recorded := range applied {
		expected := available[index]
		if recorded.Version != expected.Version {
			return nil, fmt.Errorf(
				"%w: expected applied version %06d but found %06d",
				ErrMigrationStateInvalid,
				expected.Version,
				recorded.Version,
			)
		}
		if recorded.Name != expected.Name || recorded.Checksum != expected.Checksum {
			return nil, fmt.Errorf(
				"%w: applied migration %06d does not match the embedded migration",
				ErrMigrationStateInvalid,
				expected.Version,
			)
		}
	}

	return available[len(applied):], nil
}

func validateCurrentMigrations(
	ctx context.Context,
	reader migrationStateReader,
	available []migration,
) error {
	applied, exists, err := readAppliedMigrations(ctx, reader, true)
	if err != nil {
		return err
	}
	if !exists {
		return fmt.Errorf(
			"%w: migration metadata is missing; run the migration command",
			ErrMigrationsPending,
		)
	}
	pending, err := validateMigrationState(available, applied)
	if err != nil {
		return err
	}
	if len(pending) != 0 {
		return fmt.Errorf(
			"%w: %d migration(s) must be applied",
			ErrMigrationsPending,
			len(pending),
		)
	}
	return nil
}

func readAppliedMigrations(
	ctx context.Context,
	reader migrationStateReader,
	checkTable bool,
) ([]appliedMigration, bool, error) {
	if checkTable {
		var relation sql.NullString
		if err := reader.GetContext(
			ctx,
			&relation,
			`SELECT to_regclass('schema_migrations')::text`,
		); err != nil {
			return nil, false, startupOperationError("locate schema migration metadata", ctx, err)
		}
		if !relation.Valid {
			return nil, false, nil
		}
	}

	var applied []appliedMigration
	if err := reader.SelectContext(
		ctx,
		&applied,
		`SELECT version, name, checksum FROM schema_migrations ORDER BY version ASC`,
	); err != nil {
		return nil, false, startupOperationError("read schema migration metadata", ctx, err)
	}
	return applied, true, nil
}

func applyMigrations(
	ctx context.Context,
	db *sqlx.DB,
	available []migration,
) (result MigrationResult, err error) {
	tx, err := db.BeginTxx(ctx, nil)
	if err != nil {
		return MigrationResult{}, startupOperationError("begin schema migration transaction", ctx, err)
	}
	defer func() {
		if err == nil {
			return
		}
		rollbackErr := tx.Rollback()
		if rollbackErr != nil && !errors.Is(rollbackErr, sql.ErrTxDone) {
			err = errors.Join(err, fmt.Errorf("rollback schema migrations: %w", rollbackErr))
		}
	}()

	if _, err = tx.ExecContext(
		ctx,
		`SELECT pg_advisory_xact_lock($1)`,
		migrationAdvisoryLockKey,
	); err != nil {
		return MigrationResult{}, startupOperationError("acquire schema migration lock", ctx, err)
	}
	if _, err = tx.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version BIGINT PRIMARY KEY,
			name TEXT NOT NULL,
			checksum CHAR(64) NOT NULL,
			applied_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP
		)
	`); err != nil {
		return MigrationResult{}, startupOperationError("ensure schema migration metadata", ctx, err)
	}

	applied, _, err := readAppliedMigrations(ctx, tx, false)
	if err != nil {
		return MigrationResult{}, err
	}
	pending, err := validateMigrationState(available, applied)
	if err != nil {
		return MigrationResult{}, err
	}

	for _, next := range pending {
		if _, err = tx.ExecContext(ctx, next.SQL); err != nil {
			return MigrationResult{}, startupOperationError(
				fmt.Sprintf("apply schema migration %06d_%s", next.Version, next.Name),
				ctx,
				err,
			)
		}
		if _, err = tx.ExecContext(
			ctx,
			`INSERT INTO schema_migrations (version, name, checksum) VALUES ($1, $2, $3)`,
			next.Version,
			next.Name,
			next.Checksum,
		); err != nil {
			return MigrationResult{}, startupOperationError(
				fmt.Sprintf("record schema migration %06d", next.Version),
				ctx,
				err,
			)
		}
	}

	if err = tx.Commit(); err != nil {
		return MigrationResult{}, startupOperationError("commit schema migrations", ctx, err)
	}
	return MigrationResult{
		Applied:        len(pending),
		CurrentVersion: available[len(available)-1].Version,
	}, nil
}

// MigrationResult describes a completed migration run without exposing SQL or
// connection details.
type MigrationResult struct {
	Applied        int
	CurrentVersion int64
}
