package updating

import (
	"reflect"
	"testing"
)

func TestUserUpdateCommandContainsOnlyGenericMutationFields(t *testing.T) {
	commandType := reflect.TypeOf(UserUpdateCommand{})
	wantFields := []string{
		"UserID",
		"Email",
		"PasswordDigest",
	}

	if commandType.NumField() != len(wantFields) {
		t.Fatalf("UserUpdateCommand fields = %d, want %d", commandType.NumField(), len(wantFields))
	}
	for index, wantName := range wantFields {
		field := commandType.Field(index)
		if field.Name != wantName {
			t.Fatalf("field %d = %q, want %q", index, field.Name, wantName)
		}
		if field.Tag != "" {
			t.Fatalf("field %s has transport or persistence tags %q", field.Name, field.Tag)
		}
	}

	for _, forbidden := range []string{
		"Role",
		"ExpectedRole",
		"RequestedRole",
		"Status",
		"CredentialVersion",
	} {
		if _, exists := commandType.FieldByName(forbidden); exists {
			t.Fatalf("UserUpdateCommand must not contain %s", forbidden)
		}
	}
}
