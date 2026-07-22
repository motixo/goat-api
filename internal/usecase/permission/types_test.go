package permission

import (
	"reflect"
	"testing"
)

func TestApplicationTypesDoNotDeclareTransportTags(t *testing.T) {
	types := []reflect.Type{
		reflect.TypeOf(CreateInput{}),
		reflect.TypeOf(PermissionOutput{}),
	}

	for _, typ := range types {
		for fieldIndex := 0; fieldIndex < typ.NumField(); fieldIndex++ {
			field := typ.Field(fieldIndex)
			if field.Tag != "" {
				t.Errorf("%s.%s has transport tag %q", typ.Name(), field.Name, field.Tag)
			}
		}
	}
}
