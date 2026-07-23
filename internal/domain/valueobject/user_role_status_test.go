package valueobject

import (
	"encoding/json"
	"reflect"
	"testing"
)

func TestParseUserRole(t *testing.T) {
	tests := []struct {
		input string
		want  UserRole
	}{
		{input: "client", want: RoleClient},
		{input: "operator", want: RoleOperator},
		{input: "admin", want: RoleAdmin},
	}

	for _, test := range tests {
		t.Run(test.input, func(t *testing.T) {
			got, err := ParseUserRole(test.input)
			if err != nil {
				t.Fatalf("ParseUserRole(%q) error = %v", test.input, err)
			}
			if got != test.want {
				t.Fatalf("ParseUserRole(%q) = %v, want %v", test.input, got, test.want)
			}
		})
	}
}

func TestParseUserRoleRejectsUnknownAndCaseVariants(t *testing.T) {
	for _, input := range []string{"", "unknown", "Admin", "CLIENT", " client"} {
		t.Run(input, func(t *testing.T) {
			if got, err := ParseUserRole(input); err == nil || got != RoleUnknown {
				t.Fatalf("ParseUserRole(%q) = (%v, %v), want RoleUnknown and an error", input, got, err)
			}
		})
	}
}

func TestParseUserStatus(t *testing.T) {
	tests := []struct {
		input string
		want  UserStatus
	}{
		{input: "inactive", want: StatusInactive},
		{input: "active", want: StatusActive},
		{input: "suspended", want: StatusSuspended},
	}

	for _, test := range tests {
		t.Run(test.input, func(t *testing.T) {
			got, err := ParseUserStatus(test.input)
			if err != nil {
				t.Fatalf("ParseUserStatus(%q) error = %v", test.input, err)
			}
			if got != test.want {
				t.Fatalf("ParseUserStatus(%q) = %v, want %v", test.input, got, test.want)
			}
		})
	}
}

func TestParseUserStatusRejectsUnknownAndCaseVariants(t *testing.T) {
	for _, input := range []string{"", "unknown", "Active", "SUSPENDED", " active"} {
		t.Run(input, func(t *testing.T) {
			if got, err := ParseUserStatus(input); err == nil || got != StatusUnknown {
				t.Fatalf("ParseUserStatus(%q) = (%v, %v), want StatusUnknown and an error", input, got, err)
			}
		})
	}
}

func TestUserRoleAndStatusDoNotImplementJSONUnmarshaler(t *testing.T) {
	jsonUnmarshaler := reflect.TypeOf((*json.Unmarshaler)(nil)).Elem()
	types := []reflect.Type{
		reflect.TypeOf(UserRole(0)),
		reflect.TypeOf(UserStatus(0)),
	}
	transportTags := []string{"json", "form", "query", "uri", "header", "binding", "validate"}

	for _, typ := range types {
		if reflect.PointerTo(typ).Implements(jsonUnmarshaler) {
			t.Errorf("*%s implements json.Unmarshaler; JSON decoding belongs to delivery", typ)
		}
		if typ.Kind() != reflect.Struct {
			continue
		}
		for fieldIndex := 0; fieldIndex < typ.NumField(); fieldIndex++ {
			field := typ.Field(fieldIndex)
			for _, tag := range transportTags {
				if value, ok := field.Tag.Lookup(tag); ok {
					t.Errorf("%s.%s has transport tag %s:%q", typ, field.Name, tag, value)
				}
			}
		}
	}
}
