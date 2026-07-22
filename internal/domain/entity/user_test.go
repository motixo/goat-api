package entity

import (
	"reflect"
	"testing"
)

func TestUserDoesNotDeclarePersistenceTags(t *testing.T) {
	typ := reflect.TypeOf(User{})
	for fieldIndex := 0; fieldIndex < typ.NumField(); fieldIndex++ {
		field := typ.Field(fieldIndex)
		if value, ok := field.Tag.Lookup("db"); ok {
			t.Errorf("User.%s has database tag %q", field.Name, value)
		}
	}
}
