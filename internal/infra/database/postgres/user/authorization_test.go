package user

import (
	"reflect"
	"testing"

	"github.com/motixo/goat-api/internal/domain/valueobject"
)

func TestIdentityStateRowToIdentityStateValidatesProjection(t *testing.T) {
	for _, status := range []valueobject.UserStatus{
		valueobject.StatusInactive,
		valueobject.StatusActive,
		valueobject.StatusSuspended,
	} {
		t.Run(status.String(), func(t *testing.T) {
			row := identityStateRow{
				UserID:            "user-1",
				Status:            int16(status),
				CredentialVersion: 7,
			}

			state, err := row.toIdentityState()
			if err != nil {
				t.Fatalf("toIdentityState() error = %v", err)
			}
			if state.UserID != row.UserID ||
				state.Status != status ||
				state.CredentialVersion != row.CredentialVersion {
				t.Fatalf("identity state = %#v, want validated row fields", state)
			}
		})
	}
}

func TestIdentityStateRowRejectsIncompleteOrInvalidProjection(t *testing.T) {
	valid := identityStateRow{
		UserID:            "user-1",
		Status:            int16(valueobject.StatusActive),
		CredentialVersion: 7,
	}
	tests := []struct {
		name   string
		mutate func(*identityStateRow)
	}{
		{name: "empty user ID", mutate: func(row *identityStateRow) { row.UserID = "" }},
		{name: "zero credential version", mutate: func(row *identityStateRow) { row.CredentialVersion = 0 }},
		{name: "unknown status", mutate: func(row *identityStateRow) { row.Status = 99 }},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			row := valid
			test.mutate(&row)
			if _, err := row.toIdentityState(); err == nil {
				t.Fatal("toIdentityState() error = nil for invalid projection")
			}
		})
	}
}

func TestAuthorizationStateRowToSecurityStateValidatesAndNormalizesProjection(t *testing.T) {
	statuses := []valueobject.UserStatus{
		valueobject.StatusInactive,
		valueobject.StatusActive,
		valueobject.StatusSuspended,
	}
	for _, role := range valueobject.AllRoles() {
		for _, status := range statuses {
			t.Run(role.String()+"/"+status.String(), func(t *testing.T) {
				row := authorizationStateRow{
					UserID:            "user-1",
					Status:            int16(status),
					Role:              int16(role),
					CredentialVersion: 7,
					Permissions: postgresTextArray{
						valueobject.PermUserWrite.String(),
						valueobject.PermUserRead.String(),
						valueobject.PermUserRead.String(),
					},
				}

				state, err := row.toSecurityState()
				if err != nil {
					t.Fatalf("toSecurityState() error = %v", err)
				}
				if state.UserID != row.UserID ||
					state.Status != status ||
					state.Role != role ||
					state.CredentialVersion != row.CredentialVersion {
					t.Fatalf("security state = %#v, want validated row fields", state)
				}
				want := []valueobject.Permission{
					valueobject.PermUserRead,
					valueobject.PermUserWrite,
				}
				if got := state.Permissions.Values(); !reflect.DeepEqual(got, want) {
					t.Fatalf("normalized permissions = %v, want %v", got, want)
				}
			})
		}
	}
}

func TestAuthorizationStateRowRejectsIncompleteOrInvalidProjection(t *testing.T) {
	valid := authorizationStateRow{
		UserID:            "user-1",
		Status:            int16(valueobject.StatusActive),
		Role:              int16(valueobject.RoleClient),
		CredentialVersion: 7,
	}
	tests := []struct {
		name   string
		mutate func(*authorizationStateRow)
	}{
		{name: "empty user ID", mutate: func(row *authorizationStateRow) { row.UserID = "" }},
		{name: "zero credential version", mutate: func(row *authorizationStateRow) { row.CredentialVersion = 0 }},
		{name: "unknown role", mutate: func(row *authorizationStateRow) { row.Role = 99 }},
		{name: "unknown status", mutate: func(row *authorizationStateRow) { row.Status = 99 }},
		{name: "unknown permission", mutate: func(row *authorizationStateRow) {
			row.Permissions = postgresTextArray{"unknown:permission"}
		}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			row := valid
			test.mutate(&row)
			if _, err := row.toSecurityState(); err == nil {
				t.Fatal("toSecurityState() error = nil for invalid projection")
			}
		})
	}
}
