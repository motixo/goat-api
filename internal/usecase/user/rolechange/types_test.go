package rolechange

import (
	"errors"
	"fmt"
	"reflect"
	"testing"

	"github.com/motixo/goat-api/internal/domain/valueobject"
)

func TestConcurrentRoleChangeErrorRetainsIdentityWhenWrapped(t *testing.T) {
	for _, err := range []error{
		ErrConcurrentRoleChange,
		fmt.Errorf("compare-and-set role: %w", ErrConcurrentRoleChange),
	} {
		if !errors.Is(err, ErrConcurrentRoleChange) {
			t.Fatalf("errors.Is(%v, ErrConcurrentRoleChange) = false", err)
		}
	}
}

func TestUserRoleUpdateCommandContainsOnlyRoleMutationFields(t *testing.T) {
	commandType := reflect.TypeOf(UserRoleUpdateCommand{})
	wantFields := map[string]reflect.Type{
		"UserID":        reflect.TypeOf(""),
		"ExpectedRole":  reflect.TypeOf(valueobject.UserRole(0)),
		"RequestedRole": reflect.TypeOf(valueobject.UserRole(0)),
	}

	if commandType.NumField() != len(wantFields) {
		t.Fatalf(
			"UserRoleUpdateCommand fields = %d, want %d",
			commandType.NumField(),
			len(wantFields),
		)
	}
	for index := range commandType.NumField() {
		field := commandType.Field(index)
		wantType, ok := wantFields[field.Name]
		if !ok {
			t.Fatalf("UserRoleUpdateCommand contains unexpected field %q", field.Name)
		}
		if field.Type != wantType {
			t.Fatalf(
				"UserRoleUpdateCommand.%s type = %v, want %v",
				field.Name,
				field.Type,
				wantType,
			)
		}
		if field.Tag != "" {
			t.Fatalf("UserRoleUpdateCommand.%s has outer-layer tags %q", field.Name, field.Tag)
		}
	}

	for _, forbidden := range []string{
		"PasswordDigest",
		"Email",
		"Status",
		"CredentialVersion",
	} {
		if _, exists := commandType.FieldByName(forbidden); exists {
			t.Fatalf("UserRoleUpdateCommand must not contain %s", forbidden)
		}
	}
}
