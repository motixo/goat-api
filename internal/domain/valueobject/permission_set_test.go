package valueobject

import (
	"reflect"
	"testing"
)

func TestPermissionSetNormalizesDeduplicatesAndSorts(t *testing.T) {
	set, err := NewPermissionSet([]Permission{
		PermUserUpdate,
		PermUserRead,
		PermUserUpdate,
		PermFullAccess,
	})
	if err != nil {
		t.Fatalf("NewPermissionSet() error = %v", err)
	}

	want := []Permission{
		PermFullAccess,
		PermUserRead,
		PermUserUpdate,
	}
	if got := set.Values(); !reflect.DeepEqual(got, want) {
		t.Fatalf("Values() = %v, want %v", got, want)
	}
	if !set.Has(PermUserDelete) {
		t.Fatal("full_access did not satisfy a known permission")
	}

	values := set.Values()
	values[0] = PermUserDelete
	if got := set.Values(); !reflect.DeepEqual(got, want) {
		t.Fatalf("caller mutated PermissionSet through Values(): %v", got)
	}
}

func TestPermissionSetRejectsUnknownPermission(t *testing.T) {
	if _, err := NewPermissionSet([]Permission{"database:drop"}); err == nil {
		t.Fatal("NewPermissionSet(unknown) error = nil")
	}
}

func TestAllPermissionsIsStableAndComplete(t *testing.T) {
	first := AllPermissions()
	second := AllPermissions()
	if !reflect.DeepEqual(first, second) {
		t.Fatalf("AllPermissions() is not deterministic: %v then %v", first, second)
	}
	if len(first) != 7 {
		t.Fatalf("AllPermissions() length = %d, want 7", len(first))
	}
	for _, permission := range first {
		if _, err := ParsePermission(permission.String()); err != nil {
			t.Fatalf("AllPermissions() contains unknown permission %q: %v", permission, err)
		}
	}
}
