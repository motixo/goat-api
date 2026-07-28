package valueobject

import (
	"database/sql"
	"database/sql/driver"
	"reflect"
	"testing"
)

func TestPasswordTypesDoNotImplementDatabaseInterfaces(t *testing.T) {
	valuerType := reflect.TypeOf((*driver.Valuer)(nil)).Elem()
	scannerType := reflect.TypeOf((*sql.Scanner)(nil)).Elem()

	for _, passwordType := range []reflect.Type{
		reflect.TypeOf(PlainPassword{}),
		reflect.TypeOf(PasswordDigest{}),
	} {
		if passwordType.Implements(valuerType) || reflect.PointerTo(passwordType).Implements(valuerType) {
			t.Fatalf("%s implements driver.Valuer", passwordType)
		}
		if passwordType.Implements(scannerType) || reflect.PointerTo(passwordType).Implements(scannerType) {
			t.Fatalf("%s implements sql.Scanner", passwordType)
		}
	}
}

func TestPasswordDigestEncodedPreservesStoredHash(t *testing.T) {
	const hash = "$argon2id$stored-hash"
	digest, err := NewPasswordDigest(hash)
	if err != nil {
		t.Fatalf("NewPasswordDigest() error = %v", err)
	}
	if got := digest.Encoded(); got != hash {
		t.Fatalf("PasswordDigest.Encoded() = %q, want %q", got, hash)
	}
}
