package user

import (
	"reflect"
	"testing"
)

func TestApplicationTypesDoNotDeclareTransportTags(t *testing.T) {
	types := []reflect.Type{
		reflect.TypeOf(UserOutput{}),
		reflect.TypeOf(CreateInput{}),
		reflect.TypeOf(UpdateInput{}),
		reflect.TypeOf(UpdateEmailInput{}),
		reflect.TypeOf(UpdatePassInput{}),
		reflect.TypeOf(UpdateRoleInput{}),
		reflect.TypeOf(UpdateStatusInput{}),
		reflect.TypeOf(ListFilter{}),
		reflect.TypeOf(GetListInput{}),
	}
	transportTags := []string{"json", "form", "query", "uri", "header", "binding", "validate"}

	for _, typ := range types {
		for fieldIndex := 0; fieldIndex < typ.NumField(); fieldIndex++ {
			field := typ.Field(fieldIndex)
			for _, tag := range transportTags {
				if value, ok := field.Tag.Lookup(tag); ok {
					t.Errorf("%s.%s has transport tag %s:%q", typ.Name(), field.Name, tag, value)
				}
			}
		}
	}
}
