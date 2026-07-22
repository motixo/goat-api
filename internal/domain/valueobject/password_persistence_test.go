package valueobject

import (
	"database/sql"
	"database/sql/driver"
	"reflect"
	"testing"
)

func TestPasswordDoesNotImplementDatabaseInterfaces(t *testing.T) {
	valuerType := reflect.TypeOf((*driver.Valuer)(nil)).Elem()
	scannerType := reflect.TypeOf((*sql.Scanner)(nil)).Elem()

	if reflect.TypeOf(Password{}).Implements(valuerType) {
		t.Fatal("Password implements driver.Valuer")
	}
	if reflect.TypeOf(&Password{}).Implements(scannerType) {
		t.Fatal("*Password implements sql.Scanner")
	}
}

func TestPasswordEncodedPreservesStoredHash(t *testing.T) {
	const hash = "$argon2id$stored-hash"
	if got := PasswordFromHash(hash).Encoded(); got != hash {
		t.Fatalf("Password.Encoded() = %q, want %q", got, hash)
	}
}
