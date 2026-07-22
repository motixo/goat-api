package auth

import (
	"reflect"
	"testing"
)

func TestApplicationTypesDoNotDeclareTransportTags(t *testing.T) {
	types := []reflect.Type{
		reflect.TypeOf(LoginInput{}),
		reflect.TypeOf(LoginOutput{}),
		reflect.TypeOf(UserOutput{}),
		reflect.TypeOf(RefreshInput{}),
		reflect.TypeOf(RefreshOutput{}),
		reflect.TypeOf(RegisterInput{}),
	}
	transportTags := []string{"json", "form", "query", "uri", "header", "binding", "validate"}

	for _, typ := range types {
		for i := 0; i < typ.NumField(); i++ {
			field := typ.Field(i)
			for _, tag := range transportTags {
				if value, ok := field.Tag.Lookup(tag); ok {
					t.Errorf("%s.%s has transport tag %s:%q", typ.Name(), field.Name, tag, value)
				}
			}
		}
	}
}
