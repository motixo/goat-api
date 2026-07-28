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

func TestUserRoleKnownValuesAndModificationHierarchy(t *testing.T) {
	for _, role := range AllRoles() {
		if !role.IsKnown() {
			t.Errorf("%s.IsKnown() = false, want true", role)
		}
	}
	for _, role := range []UserRole{RoleUnknown, UserRole(255)} {
		if role.IsKnown() {
			t.Errorf("UserRole(%d).IsKnown() = true, want false", role)
		}
	}

	tests := []struct {
		actor  UserRole
		target UserRole
		want   bool
	}{
		{actor: RoleAdmin, target: RoleAdmin, want: true},
		{actor: RoleAdmin, target: RoleOperator, want: true},
		{actor: RoleAdmin, target: RoleClient, want: true},
		{actor: RoleOperator, target: RoleAdmin, want: false},
		{actor: RoleOperator, target: RoleOperator, want: false},
		{actor: RoleOperator, target: RoleClient, want: true},
		{actor: RoleClient, target: RoleClient, want: false},
	}
	for _, test := range tests {
		if got := test.actor.CanModifyTargetRole(test.target); got != test.want {
			t.Errorf(
				"%s.CanModifyTargetRole(%s) = %t, want %t",
				test.actor,
				test.target,
				got,
				test.want,
			)
		}
		if got := test.actor.CanAssignRole(test.target); got != test.want {
			t.Errorf(
				"%s.CanAssignRole(%s) = %t, want %t",
				test.actor,
				test.target,
				got,
				test.want,
			)
		}
	}
	for _, requested := range []UserRole{RoleUnknown, UserRole(255)} {
		if RoleAdmin.CanAssignRole(requested) {
			t.Errorf("admin CanAssignRole(UserRole(%d)) = true, want false", requested)
		}
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

func TestUserStatusTransitionMatrix(t *testing.T) {
	statuses := []UserStatus{
		StatusUnknown,
		StatusInactive,
		StatusActive,
		StatusSuspended,
		UserStatus(255),
	}
	allowed := map[[2]UserStatus]bool{
		{StatusInactive, StatusInactive}:   true,
		{StatusInactive, StatusActive}:     true,
		{StatusActive, StatusActive}:       true,
		{StatusActive, StatusSuspended}:    true,
		{StatusSuspended, StatusSuspended}: true,
		{StatusSuspended, StatusActive}:    true,
	}

	for _, current := range statuses {
		for _, next := range statuses {
			want := allowed[[2]UserStatus{current, next}]
			if got := current.CanTransitionTo(next); got != want {
				t.Errorf(
					"%s.CanTransitionTo(%s) = %t, want %t",
					current,
					next,
					got,
					want,
				)
			}
		}
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
