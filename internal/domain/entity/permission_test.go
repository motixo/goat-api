package entity

import (
	"reflect"
	"testing"
)

func TestPermissionDoesNotDeclarePersistenceTags(t *testing.T) {
	typeOfPermission := reflect.TypeOf(Permission{})

	for fieldIndex := 0; fieldIndex < typeOfPermission.NumField(); fieldIndex++ {
		field := typeOfPermission.Field(fieldIndex)
		if tag := field.Tag.Get("db"); tag != "" {
			t.Errorf("Permission.%s has database tag %q", field.Name, tag)
		}
	}
}
