package listing

import (
	"reflect"
	"testing"
)

func TestUserListApplicationTypesAreCredentialAndAdapterFree(t *testing.T) {
	assertFields(t, reflect.TypeOf(UserListItem{}), []string{
		"ID",
		"Email",
		"Role",
		"Status",
		"CreatedAt",
	})
	assertFields(t, reflect.TypeOf(UserListCriteria{}), []string{
		"Statuses",
		"Roles",
		"Search",
	})
	assertFields(t, reflect.TypeOf(UserListResult{}), []string{
		"Items",
		"Total",
	})
}

func assertFields(t *testing.T, typ reflect.Type, expected []string) {
	t.Helper()
	if typ.NumField() != len(expected) {
		t.Fatalf("%s field count = %d, want %d", typ.Name(), typ.NumField(), len(expected))
	}

	for index, name := range expected {
		field := typ.Field(index)
		if field.Name != name {
			t.Fatalf("%s field %d = %q, want %q", typ.Name(), index, field.Name, name)
		}
		if field.Tag != "" {
			t.Fatalf("%s.%s has transport or persistence tags %q", typ.Name(), field.Name, field.Tag)
		}
	}
}
