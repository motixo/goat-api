package permission

import (
	"database/sql"
	"errors"
	"fmt"
	"testing"

	"github.com/lib/pq"
	domainErrors "github.com/motixo/goat-api/internal/domain/errors"
)

func TestTranslatePermissionDeleteError(t *testing.T) {
	t.Run("no rows preserves semantic identity and cause", func(t *testing.T) {
		for _, err := range []error{
			sql.ErrNoRows,
			fmt.Errorf("query permission: %w", sql.ErrNoRows),
		} {
			got := translatePermissionDeleteError(err)
			if !errors.Is(got, domainErrors.ErrPermissionNotFound) {
				t.Fatalf("translated error = %v, want ErrPermissionNotFound", got)
			}
			if !errors.Is(got, sql.ErrNoRows) {
				t.Fatalf("translated error = %v, want wrapped sql.ErrNoRows", got)
			}
		}
	})

	t.Run("unknown failures remain unchanged", func(t *testing.T) {
		cause := errors.New("database unavailable")
		if got := translatePermissionDeleteError(cause); got != cause {
			t.Fatalf("translated error = %v, want original error %v", got, cause)
		}
	})

	t.Run("nil remains nil", func(t *testing.T) {
		if got := translatePermissionDeleteError(nil); got != nil {
			t.Fatalf("translated error = %v, want nil", got)
		}
	})
}

func TestTranslatePermissionCreateError(t *testing.T) {
	t.Run("role action uniqueness preserves semantic identity and postgres cause", func(t *testing.T) {
		cause := &pq.Error{
			Code:       permissionUniqueViolation,
			Constraint: permissionRoleActionUniqueConstraint,
		}
		for _, err := range []error{
			cause,
			fmt.Errorf("insert permission: %w", cause),
		} {
			got := translatePermissionCreateError(err)
			if !errors.Is(got, domainErrors.ErrPermissionAlreadyExists) {
				t.Fatalf("translated error = %v, want ErrPermissionAlreadyExists", got)
			}
			var gotCause *pq.Error
			if !errors.As(got, &gotCause) {
				t.Fatalf("translated error = %v, want wrapped *pq.Error", got)
			}
			if gotCause != cause {
				t.Fatalf("wrapped PostgreSQL cause = %p, want %p", gotCause, cause)
			}
		}
	})

	t.Run("other postgres constraints remain unknown", func(t *testing.T) {
		cause := &pq.Error{
			Code:       permissionUniqueViolation,
			Constraint: "permissions_pkey",
		}
		got := translatePermissionCreateError(cause)
		if got != cause {
			t.Fatalf("translated error = %v, want original error %v", got, cause)
		}
		if errors.Is(got, domainErrors.ErrPermissionAlreadyExists) {
			t.Fatalf("translated error = %v, must not classify an unrelated constraint", got)
		}
	})

	t.Run("unknown failures remain unchanged", func(t *testing.T) {
		cause := errors.New("database unavailable")
		if got := translatePermissionCreateError(cause); got != cause {
			t.Fatalf("translated error = %v, want original error %v", got, cause)
		}
	})

	t.Run("nil remains nil", func(t *testing.T) {
		if got := translatePermissionCreateError(nil); got != nil {
			t.Fatalf("translated error = %v, want nil", got)
		}
	})
}
