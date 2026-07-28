package statuschange

import (
	"reflect"
	"testing"

	"github.com/motixo/goat-api/internal/domain/valueobject"
)

func TestUserStatusSnapshotContainsOnlyCommandPreconditions(t *testing.T) {
	typeOfSnapshot := reflect.TypeOf(UserStatusSnapshot{})
	wantFields := map[string]reflect.Type{
		"ID":     reflect.TypeOf(""),
		"Role":   reflect.TypeOf(valueobject.UserRole(0)),
		"Status": reflect.TypeOf(valueobject.UserStatus(0)),
	}

	if typeOfSnapshot.NumField() != len(wantFields) {
		t.Fatalf(
			"UserStatusSnapshot field count = %d, want %d",
			typeOfSnapshot.NumField(),
			len(wantFields),
		)
	}
	for index := range typeOfSnapshot.NumField() {
		field := typeOfSnapshot.Field(index)
		wantType, ok := wantFields[field.Name]
		if !ok {
			t.Fatalf("UserStatusSnapshot contains unexpected field %q", field.Name)
		}
		if field.Type != wantType {
			t.Fatalf(
				"UserStatusSnapshot.%s type = %v, want %v",
				field.Name,
				field.Type,
				wantType,
			)
		}
		if field.Tag != "" {
			t.Fatalf("UserStatusSnapshot.%s has outer-layer tags %q", field.Name, field.Tag)
		}
	}
}
