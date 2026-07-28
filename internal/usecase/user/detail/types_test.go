package detail

import (
	"reflect"
	"testing"
)

func TestUserDetailIsCredentialFreeAndLayerIndependent(t *testing.T) {
	typ := reflect.TypeOf(UserDetail{})

	wantFields := []string{"ID", "Email", "Role", "Status", "CreatedAt"}
	if typ.NumField() != len(wantFields) {
		t.Fatalf("UserDetail field count = %d, want %d", typ.NumField(), len(wantFields))
	}

	outerLayerTags := []string{"json", "form", "query", "uri", "header", "binding", "validate", "db"}
	for index, wantName := range wantFields {
		field := typ.Field(index)
		if field.Name != wantName {
			t.Fatalf("UserDetail field %d = %q, want %q", index, field.Name, wantName)
		}
		for _, tag := range outerLayerTags {
			if value, ok := field.Tag.Lookup(tag); ok {
				t.Errorf("UserDetail.%s has outer-layer tag %s:%q", field.Name, tag, value)
			}
		}
	}

	for _, forbidden := range []string{"Password", "PasswordDigest", "CredentialVersion", "UpdatedAt"} {
		if _, exists := typ.FieldByName(forbidden); exists {
			t.Errorf("UserDetail contains unrelated or credential field %q", forbidden)
		}
	}
}
