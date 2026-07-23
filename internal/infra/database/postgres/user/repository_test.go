package user

import (
	"database/sql"
	"errors"
	"fmt"
	"testing"

	"github.com/lib/pq"
	domainErrors "github.com/motixo/goat-api/internal/domain/errors"
)

func TestBuildUserListSelectQueryUsesStableTieBreaker(t *testing.T) {
	query := buildUserListSelectQuery(" WHERE role = ANY($1)", 2)
	want := "SELECT id, email, role, status, credential_version, created_at, updated_at FROM users" +
		" WHERE role = ANY($1)" +
		" ORDER BY created_at DESC, id DESC LIMIT $2 OFFSET $3"

	if query != want {
		t.Fatalf("query = %q, want %q", query, want)
	}
}

func TestTranslateUserWriteError(t *testing.T) {
	emailConflict := &pq.Error{
		Code:       "23505",
		Constraint: "users_email_key",
		Message:    "duplicate key value violates unique constraint",
	}

	for _, test := range []struct {
		name         string
		err          error
		wantSemantic bool
		wantSame     bool
		wantCause    *pq.Error
	}{
		{
			name:         "email unique violation",
			err:          emailConflict,
			wantSemantic: true,
			wantCause:    emailConflict,
		},
		{
			name:         "wrapped email unique violation",
			err:          fmt.Errorf("execute insert: %w", emailConflict),
			wantSemantic: true,
			wantCause:    emailConflict,
		},
		{
			name: "unrelated unique constraint",
			err: &pq.Error{
				Code:       "23505",
				Constraint: "users_pkey",
			},
			wantSame: true,
		},
		{
			name: "same constraint with unrelated SQLSTATE",
			err: &pq.Error{
				Code:       "23503",
				Constraint: "users_email_key",
			},
			wantSame: true,
		},
		{
			name:     "unknown persistence failure",
			err:      errors.New("postgres unavailable"),
			wantSame: true,
		},
		{
			name:     "nil",
			wantSame: true,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			got := translateUserWriteError(test.err)

			if errors.Is(got, domainErrors.ErrEmailAlreadyExists) != test.wantSemantic {
				t.Fatalf("errors.Is(ErrEmailAlreadyExists) = %t, want %t; error = %v",
					errors.Is(got, domainErrors.ErrEmailAlreadyExists), test.wantSemantic, got)
			}
			if test.wantSame && got != test.err {
				t.Fatalf("translateUserWriteError() = %v, want original error %v", got, test.err)
			}
			if test.wantCause != nil {
				var postgresErr *pq.Error
				if !errors.As(got, &postgresErr) {
					t.Fatalf("translated error does not preserve *pq.Error: %v", got)
				}
				if postgresErr != test.wantCause {
					t.Fatalf("preserved PostgreSQL error = %p, want %p", postgresErr, test.wantCause)
				}
			}
		})
	}
}

func TestTranslateUserFindByIDError(t *testing.T) {
	wrappedNoRows := fmt.Errorf("select user by ID: %w", sql.ErrNoRows)
	unknownErr := errors.New("postgres connection unavailable")

	for _, test := range []struct {
		name         string
		err          error
		wantSemantic bool
		wantNoRows   bool
		wantSame     bool
	}{
		{
			name:         "direct no rows",
			err:          sql.ErrNoRows,
			wantSemantic: true,
			wantNoRows:   true,
		},
		{
			name:         "wrapped no rows",
			err:          wrappedNoRows,
			wantSemantic: true,
			wantNoRows:   true,
		},
		{
			name:     "unknown lookup failure",
			err:      unknownErr,
			wantSame: true,
		},
		{
			name:     "nil",
			wantSame: true,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			got := translateUserFindByIDError(test.err)

			if errors.Is(got, domainErrors.ErrUserNotFound) != test.wantSemantic {
				t.Fatalf("errors.Is(ErrUserNotFound) = %t, want %t; error = %v",
					errors.Is(got, domainErrors.ErrUserNotFound), test.wantSemantic, got)
			}
			if errors.Is(got, sql.ErrNoRows) != test.wantNoRows {
				t.Fatalf("errors.Is(sql.ErrNoRows) = %t, want %t; error = %v",
					errors.Is(got, sql.ErrNoRows), test.wantNoRows, got)
			}
			if test.wantSame && got != test.err {
				t.Fatalf("translateUserFindByIDError() = %v, want original error %v", got, test.err)
			}
		})
	}
}
