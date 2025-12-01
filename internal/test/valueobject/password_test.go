package valueobject_test

import (
	"testing"

	"github.com/mot0x0/goth-api/internal/domain/valueobject"
)

func TestPasswordFromHashAndValue(t *testing.T) {
	hash := "hashedpassword123!@#"
	p := valueobject.PasswordFromHash(hash)

	if p.Value() != hash {
		t.Errorf("expected Value() %s, got %s", hash, p.Value())
	}
}

func TestPassword_IsZero(t *testing.T) {
	empty := valueobject.Password{}
	if !empty.IsZero() {
		t.Error("expected IsZero() to return true for empty password")
	}

	nonEmpty := valueobject.PasswordFromHash("nonempty")
	if nonEmpty.IsZero() {
		t.Error("expected IsZero() to return false for non-empty password")
	}
}
