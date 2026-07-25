package authorization

import (
	"context"
	"errors"
	"fmt"
	"testing"

	domainErrors "github.com/motixo/goat-api/internal/domain/errors"
	"github.com/motixo/goat-api/internal/domain/valueobject"
)

func TestAuthorizeFreshUsesOneAuthoritativeStateReadAndOverridesSnapshot(t *testing.T) {
	currentPermissions, err := valueobject.NewPermissionSet([]valueobject.Permission{
		valueobject.PermUserRead,
	})
	if err != nil {
		t.Fatalf("build current permission set: %v", err)
	}
	stalePermissions, err := valueobject.NewPermissionSet([]valueobject.Permission{
		valueobject.PermFullAccess,
	})
	if err != nil {
		t.Fatalf("build stale permission set: %v", err)
	}
	reader := &recordingSecurityStateReader{
		state: SecurityState{
			UserID:            "user-1",
			Status:            valueobject.StatusActive,
			Role:              valueobject.RoleOperator,
			CredentialVersion: 7,
			Permissions:       currentPermissions,
		},
	}
	principal, err := NewPrincipal(
		"user-1",
		"session-1",
		7,
		valueobject.RoleAdmin,
		stalePermissions,
	)
	if err != nil {
		t.Fatalf("NewPrincipal() error = %v", err)
	}

	fresh, err := NewUsecase(nil, reader).AuthorizeFresh(
		context.Background(),
		principal,
		valueobject.PermUserRead,
	)
	if err != nil {
		t.Fatalf("AuthorizeFresh() error = %v", err)
	}
	if reader.calls != 1 {
		t.Fatalf("authoritative state reads = %d, want 1", reader.calls)
	}
	if fresh.Role() != valueobject.RoleOperator {
		t.Fatalf("fresh role = %s, want operator", fresh.Role())
	}
	if fresh.Permissions().Has(valueobject.PermFullAccess) {
		t.Fatal("fresh principal retained stale full_access claim")
	}
}

func TestAuthorizeFreshImmediatelyObservesNewlyGrantedPermission(t *testing.T) {
	stalePermissions, err := valueobject.NewPermissionSet(nil)
	if err != nil {
		t.Fatalf("build stale permission set: %v", err)
	}
	currentPermissions, err := valueobject.NewPermissionSet([]valueobject.Permission{
		valueobject.PermUserDelete,
	})
	if err != nil {
		t.Fatalf("build current permission set: %v", err)
	}
	principal, err := NewPrincipal(
		"user-1",
		"session-1",
		7,
		valueobject.RoleOperator,
		stalePermissions,
	)
	if err != nil {
		t.Fatalf("NewPrincipal() error = %v", err)
	}
	reader := &recordingSecurityStateReader{state: SecurityState{
		UserID: "user-1", Status: valueobject.StatusActive,
		Role: valueobject.RoleOperator, CredentialVersion: 7,
		Permissions: currentPermissions,
	}}

	fresh, err := NewUsecase(nil, reader).AuthorizeFresh(
		context.Background(),
		principal,
		valueobject.PermUserDelete,
	)
	if err != nil {
		t.Fatalf("AuthorizeFresh() error = %v", err)
	}
	if reader.calls != 1 {
		t.Fatalf("authoritative state reads = %d, want 1", reader.calls)
	}
	if !fresh.Permissions().Has(valueobject.PermUserDelete) {
		t.Fatal("fresh principal did not include the newly granted permission")
	}
}

func TestAuthorizeFreshFailsClosedForCurrentSecurityState(t *testing.T) {
	permissions, err := valueobject.NewPermissionSet([]valueobject.Permission{
		valueobject.PermUserRead,
	})
	if err != nil {
		t.Fatalf("build permission set: %v", err)
	}
	principal, err := NewPrincipal(
		"user-1",
		"session-1",
		7,
		valueobject.RoleOperator,
		permissions,
	)
	if err != nil {
		t.Fatalf("NewPrincipal() error = %v", err)
	}
	postgresErr := errors.New("postgres unavailable")

	tests := []struct {
		name      string
		state     SecurityState
		readerErr error
		required  valueobject.Permission
		want      error
	}{
		{
			name: "inactive",
			state: SecurityState{
				UserID: "user-1", Status: valueobject.StatusInactive,
				Role: valueobject.RoleOperator, CredentialVersion: 7, Permissions: permissions,
			},
			want: ErrPrincipalInactive,
		},
		{
			name: "suspended",
			state: SecurityState{
				UserID: "user-1", Status: valueobject.StatusSuspended,
				Role: valueobject.RoleOperator, CredentialVersion: 7, Permissions: permissions,
			},
			want: ErrPrincipalSuspended,
		},
		{
			name: "credential version mismatch",
			state: SecurityState{
				UserID: "user-1", Status: valueobject.StatusActive,
				Role: valueobject.RoleOperator, CredentialVersion: 8, Permissions: permissions,
			},
			want: ErrPrincipalSecurityStateChanged,
		},
		{
			name: "deleted",
			readerErr: fmt.Errorf(
				"load security state: %w",
				domainErrors.ErrUserNotFound,
			),
			want: ErrPrincipalSecurityStateChanged,
		},
		{
			name: "removed permission",
			state: SecurityState{
				UserID: "user-1", Status: valueobject.StatusActive,
				Role: valueobject.RoleOperator, CredentialVersion: 7, Permissions: permissions,
			},
			required: valueobject.PermUserDelete,
			want:     ErrPermissionDenied,
		},
		{
			name:      "unknown postgres failure",
			readerErr: postgresErr,
			want:      postgresErr,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			reader := &recordingSecurityStateReader{
				state: test.state,
				err:   test.readerErr,
			}
			_, err := NewUsecase(nil, reader).AuthorizeFresh(
				context.Background(),
				principal,
				test.required,
			)
			if !errors.Is(err, test.want) {
				t.Fatalf("AuthorizeFresh() error = %v, want %v", err, test.want)
			}
			if reader.calls != 1 {
				t.Fatalf("authoritative state reads = %d, want 1", reader.calls)
			}
		})
	}
}

func TestAuthorizeFreshIdentityUsesOneProjectionAndNoPermissionDecision(t *testing.T) {
	permissions, err := valueobject.NewPermissionSet(nil)
	if err != nil {
		t.Fatalf("build permissions: %v", err)
	}
	principal, err := NewPrincipal(
		"user-1",
		"session-1",
		7,
		valueobject.RoleClient,
		permissions,
	)
	if err != nil {
		t.Fatalf("NewPrincipal() error = %v", err)
	}
	reader := &recordingIdentityStateReader{state: IdentityState{
		UserID:            "user-1",
		Status:            valueobject.StatusActive,
		CredentialVersion: 7,
	}}

	fresh, err := NewUsecase(reader, nil).AuthorizeFreshIdentity(
		context.Background(),
		principal,
	)
	if err != nil {
		t.Fatalf("AuthorizeFreshIdentity() error = %v", err)
	}
	if reader.calls != 1 {
		t.Fatalf("identity-state reads = %d, want 1", reader.calls)
	}
	if fresh.Role() != valueobject.RoleClient ||
		len(fresh.Permissions().Values()) != 0 {
		t.Fatalf(
			"fresh identity changed authorization snapshot: role=%s permissions=%v",
			fresh.Role(),
			fresh.Permissions().Values(),
		)
	}
}

func TestAuthorizeFreshIdentityFailsClosed(t *testing.T) {
	permissions, err := valueobject.NewPermissionSet(nil)
	if err != nil {
		t.Fatalf("build permissions: %v", err)
	}
	principal, err := NewPrincipal(
		"user-1",
		"session-1",
		7,
		valueobject.RoleClient,
		permissions,
	)
	if err != nil {
		t.Fatalf("NewPrincipal() error = %v", err)
	}
	postgresErr := errors.New("postgres unavailable")

	for _, test := range []struct {
		name      string
		state     IdentityState
		readerErr error
		want      error
	}{
		{
			name: "inactive",
			state: IdentityState{
				UserID: "user-1", Status: valueobject.StatusInactive,
				CredentialVersion: 7,
			},
			want: ErrPrincipalInactive,
		},
		{
			name: "suspended",
			state: IdentityState{
				UserID: "user-1", Status: valueobject.StatusSuspended,
				CredentialVersion: 7,
			},
			want: ErrPrincipalSuspended,
		},
		{
			name: "version changed",
			state: IdentityState{
				UserID: "user-1", Status: valueobject.StatusActive,
				CredentialVersion: 8,
			},
			want: ErrPrincipalSecurityStateChanged,
		},
		{
			name:      "deleted",
			readerErr: fmt.Errorf("identity lookup: %w", domainErrors.ErrUserNotFound),
			want:      ErrPrincipalSecurityStateChanged,
		},
		{
			name:      "unknown PostgreSQL error",
			readerErr: postgresErr,
			want:      postgresErr,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			reader := &recordingIdentityStateReader{
				state: test.state,
				err:   test.readerErr,
			}
			_, err := NewUsecase(reader, nil).AuthorizeFreshIdentity(
				context.Background(),
				principal,
			)
			if !errors.Is(err, test.want) {
				t.Fatalf(
					"AuthorizeFreshIdentity() error = %v, want %v",
					err,
					test.want,
				)
			}
			if reader.calls != 1 {
				t.Fatalf("identity-state reads = %d, want 1", reader.calls)
			}
		})
	}
}

func TestAuthorizationErrorsRetainIdentityWhenWrapped(t *testing.T) {
	for _, identity := range []error{
		ErrPrincipalInactive,
		ErrPrincipalSuspended,
		ErrPrincipalSecurityStateChanged,
		ErrPermissionDenied,
	} {
		if !errors.Is(fmt.Errorf("authorize request: %w", identity), identity) {
			t.Fatalf("wrapped identity %v was not preserved", identity)
		}
	}
}

type recordingSecurityStateReader struct {
	state SecurityState
	err   error
	calls int
}

func (r *recordingSecurityStateReader) GetSecurityState(
	context.Context,
	string,
) (SecurityState, error) {
	r.calls++
	return r.state, r.err
}

type recordingIdentityStateReader struct {
	state IdentityState
	err   error
	calls int
}

func (r *recordingIdentityStateReader) GetIdentityState(
	context.Context,
	string,
) (IdentityState, error) {
	r.calls++
	return r.state, r.err
}
