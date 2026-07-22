package repository

import (
	"reflect"
	"testing"
)

func TestUserListFilterDoesNotDeclareOuterLayerTags(t *testing.T) {
	typ := reflect.TypeOf(UserListFilter{})
	outerLayerTags := []string{"json", "form", "query", "uri", "header", "binding", "validate", "db"}

	for fieldIndex := 0; fieldIndex < typ.NumField(); fieldIndex++ {
		field := typ.Field(fieldIndex)
		for _, tag := range outerLayerTags {
			if value, ok := field.Tag.Lookup(tag); ok {
				t.Errorf("%s.%s has outer-layer tag %s:%q", typ.Name(), field.Name, tag, value)
			}
		}
	}
}
