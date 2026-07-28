package emailchange

import (
	"reflect"
	"testing"
)

func TestUserEmailUpdateCommandContainsOnlyEmailMutationState(t *testing.T) {
	commandType := reflect.TypeOf(UserEmailUpdateCommand{})
	if commandType.NumField() != 2 {
		t.Fatalf("UserEmailUpdateCommand fields = %d, want 2", commandType.NumField())
	}

	for _, fieldName := range []string{"UserID", "Email"} {
		field, exists := commandType.FieldByName(fieldName)
		if !exists {
			t.Fatalf("UserEmailUpdateCommand missing %s", fieldName)
		}
		if field.Tag != "" {
			t.Fatalf("UserEmailUpdateCommand.%s tag = %q, want none", fieldName, field.Tag)
		}
	}
}
