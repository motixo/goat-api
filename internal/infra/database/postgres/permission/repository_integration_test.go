package permission

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/lib/pq"
	"github.com/motixo/goat-api/internal/domain/entity"
	domainErrors "github.com/motixo/goat-api/internal/domain/errors"
	"github.com/motixo/goat-api/internal/domain/repository"
	"github.com/motixo/goat-api/internal/domain/valueobject"
)

func TestRepositoryIntegrationAllOperations(t *testing.T) {
	repo := newPostgresPermissionIntegrationRepository(t)
	ctx := context.Background()
	createdAt := time.Date(2026, time.July, 23, 17, 0, 0, 123000000, time.UTC)
	permissions := []*entity.Permission{
		integrationPermission("11111111-1111-4111-8111-111111111111", valueobject.RoleAdmin, valueobject.PermFullAccess, createdAt),
		integrationPermission("22222222-2222-4222-8222-222222222222", valueobject.RoleOperator, valueobject.PermUserRead, createdAt.Add(time.Minute)),
		integrationPermission("33333333-3333-4333-8333-333333333333", valueobject.RoleClient, valueobject.PermUserRead, createdAt.Add(2*time.Minute)),
	}
	for _, permission := range permissions {
		if err := repo.Create(ctx, permission); err != nil {
			t.Fatalf("Create(%s) error = %v", permission.ID, err)
		}
	}

	listed, total, err := repo.List(ctx, 0, 10)
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if total != int64(len(permissions)) {
		t.Fatalf("List() total = %d, want %d", total, len(permissions))
	}
	assertPermissionIDs(t, listed, []string{
		"11111111-1111-4111-8111-111111111111",
		"22222222-2222-4222-8222-222222222222",
		"33333333-3333-4333-8333-333333333333",
	})
	for index := range listed {
		assertPersistedPermissionEqual(t, listed[index], permissions[index])
	}

	secondPage, total, err := repo.List(ctx, 1, 1)
	if err != nil {
		t.Fatalf("List(second page) error = %v", err)
	}
	if total != int64(len(permissions)) {
		t.Fatalf("List(second page) total = %d, want %d", total, len(permissions))
	}
	assertPermissionIDs(t, secondPage, []string{"22222222-2222-4222-8222-222222222222"})

	byRole, err := repo.GetByRoleID(ctx, valueobject.RoleOperator)
	if err != nil {
		t.Fatalf("GetByRoleID() error = %v", err)
	}
	if len(byRole) != 1 {
		t.Fatalf("GetByRoleID() count = %d, want 1", len(byRole))
	}
	assertPersistedPermissionEqual(t, byRole[0], permissions[1])

	additional := integrationPermission(
		"44444444-4444-4444-8444-444444444444",
		valueobject.RoleOperator,
		valueobject.PermUserUpdate,
		createdAt.Add(3*time.Minute),
	)
	if err := repo.Create(ctx, additional); err != nil {
		t.Fatalf("Create(additional operator permission) error = %v", err)
	}
	byRole, err = repo.GetByRoleID(ctx, valueobject.RoleOperator)
	if err != nil {
		t.Fatalf("GetByRoleID(multiple) error = %v", err)
	}
	assertPermissionSet(t, byRole, map[string]*entity.Permission{
		permissions[1].ID: permissions[1],
		additional.ID:     additional,
	})

	duplicate := integrationPermission(
		"55555555-5555-4555-8555-555555555555",
		additional.Role,
		additional.Action,
		createdAt.Add(4*time.Minute),
	)
	err = repo.Create(ctx, duplicate)
	if !errors.Is(err, domainErrors.ErrPermissionAlreadyExists) {
		t.Fatalf("Create(duplicate role/action) error = %v, want ErrPermissionAlreadyExists", err)
	}
	var pqErr *pq.Error
	if !errors.As(err, &pqErr) {
		t.Fatalf("Create(duplicate role/action) error = %v, want wrapped *pq.Error", err)
	}
	if pqErr.Code != "23505" || pqErr.Constraint != "unique_role_action" {
		t.Fatalf("Create(duplicate role/action) PostgreSQL error = code %q constraint %q", pqErr.Code, pqErr.Constraint)
	}

	deletedRole, err := repo.Delete(ctx, additional.ID)
	if err != nil {
		t.Fatalf("Delete() error = %v", err)
	}
	if deletedRole != int8(additional.Role) {
		t.Fatalf("Delete() role = %d, want %d", deletedRole, additional.Role)
	}
	byRole, err = repo.GetByRoleID(ctx, valueobject.RoleOperator)
	if err != nil {
		t.Fatalf("GetByRoleID(after delete) error = %v", err)
	}
	if len(byRole) != 1 || byRole[0].ID != permissions[1].ID {
		t.Fatalf("GetByRoleID(after delete) = %#v, want only %s", byRole, permissions[1].ID)
	}

	_, err = repo.Delete(ctx, additional.ID)
	if !errors.Is(err, domainErrors.ErrPermissionNotFound) {
		t.Fatalf("Delete(missing) error = %v, want ErrPermissionNotFound", err)
	}
	if !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("Delete(missing) error = %v, want wrapped sql.ErrNoRows", err)
	}
}

func newPostgresPermissionIntegrationRepository(t *testing.T) repository.PermissionRepository {
	t.Helper()
	db := newPostgresPermissionIntegrationDB(t)
	return NewRepository(db)
}

func newPostgresPermissionIntegrationDB(t *testing.T) *sqlx.DB {
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

	const schema = `
		CREATE TEMP TABLE permissions (
			id UUID PRIMARY KEY,
			role SMALLINT NOT NULL,
			action TEXT NOT NULL,
			created_at TIMESTAMP NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMP NULL,
			CONSTRAINT unique_role_action UNIQUE(role, action)
		) ON COMMIT PRESERVE ROWS
	`
	if _, err := db.Exec(schema); err != nil {
		t.Fatalf("create temporary permissions table: %v", err)
	}
	return db
}

func integrationPermission(id string, role valueobject.UserRole, action valueobject.Permission, createdAt time.Time) *entity.Permission {
	return &entity.Permission{
		ID:        id,
		Role:      role,
		Action:    action,
		CreatedAt: createdAt,
	}
}

func assertPersistedPermissionEqual(t *testing.T, got, want *entity.Permission) {
	t.Helper()
	if got == nil {
		t.Fatal("persisted permission is nil")
	}
	if got.ID != want.ID || got.Role != want.Role || got.Action != want.Action || !got.CreatedAt.Equal(want.CreatedAt) {
		t.Fatalf("persisted permission = %#v, want %#v", got, want)
	}
}

func assertPermissionIDs(t *testing.T, permissions []*entity.Permission, want []string) {
	t.Helper()
	if len(permissions) != len(want) {
		t.Fatalf("permission count = %d, want %d", len(permissions), len(want))
	}
	for index := range want {
		if permissions[index].ID != want[index] {
			t.Fatalf("permissions[%d].ID = %q, want %q", index, permissions[index].ID, want[index])
		}
	}
}

func assertPermissionSet(t *testing.T, got []*entity.Permission, want map[string]*entity.Permission) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("permission count = %d, want %d", len(got), len(want))
	}
	for _, permission := range got {
		wantPermission, exists := want[permission.ID]
		if !exists {
			t.Fatalf("unexpected permission ID %q", permission.ID)
		}
		assertPersistedPermissionEqual(t, permission, wantPermission)
	}
}
