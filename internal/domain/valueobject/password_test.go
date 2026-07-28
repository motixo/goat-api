package valueobject

import (
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"

	domainErrors "github.com/motixo/goat-api/internal/domain/errors"
)

func TestNewPlainPasswordAlwaysAppliesPasswordPolicy(t *testing.T) {
	tests := []struct {
		name      string
		value     string
		wantErr   error
		wantValid bool
	}{
		{name: "valid", value: "Password1!", wantValid: true},
		{name: "valid leading dollar", value: "$Password1!", wantValid: true},
		{name: "valid hash-looking plaintext", value: "$argon2id$Password1!", wantValid: true},
		{name: "short leading dollar", value: "$A1!", wantErr: domainErrors.ErrPasswordTooShort},
		{name: "hash prefix does not bypass policy", value: "$argon2id$", wantErr: domainErrors.ErrPasswordPolicyViolation},
		{name: "too long", value: strings.Repeat("A1!a", 19), wantErr: domainErrors.ErrPasswordTooLong},
		{name: "missing uppercase", value: "password1!", wantErr: domainErrors.ErrPasswordPolicyViolation},
		{name: "missing lowercase", value: "PASSWORD1!", wantErr: domainErrors.ErrPasswordPolicyViolation},
		{name: "missing number", value: "Password!", wantErr: domainErrors.ErrPasswordPolicyViolation},
		{name: "missing special", value: "Password1", wantErr: domainErrors.ErrPasswordPolicyViolation},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			password, err := NewPlainPassword(test.value)
			if test.wantErr != nil {
				if !errors.Is(err, test.wantErr) {
					t.Fatalf("NewPlainPassword() error = %v, want %v", err, test.wantErr)
				}
				if !password.IsZero() {
					t.Fatal("NewPlainPassword() returned a non-zero value after validation failure")
				}
				return
			}
			if err != nil {
				t.Fatalf("NewPlainPassword() error = %v", err)
			}
			if !test.wantValid || password.IsZero() {
				t.Fatal("NewPlainPassword() did not return a valid plaintext value")
			}
			if got := string(password.Bytes()); got != test.value {
				t.Fatalf("PlainPassword.Bytes() = %q, want original value", got)
			}
		})
	}
}

func TestPasswordDigestRehydrationIsOpaqueButRejectsEmpty(t *testing.T) {
	const encoded = "$not-an-argon-format-but-still-an-opaque-persisted-value"
	digest, err := NewPasswordDigest(encoded)
	if err != nil {
		t.Fatalf("NewPasswordDigest() error = %v", err)
	}
	if digest.IsZero() {
		t.Fatal("NewPasswordDigest() returned a zero digest")
	}
	if got := digest.Encoded(); got != encoded {
		t.Fatalf("PasswordDigest.Encoded() = %q, want opaque value", got)
	}

	empty, err := NewPasswordDigest("")
	if !errors.Is(err, domainErrors.ErrInvalidStoredPasswordHash) {
		t.Fatalf("NewPasswordDigest(empty) error = %v, want ErrInvalidStoredPasswordHash", err)
	}
	if !empty.IsZero() {
		t.Fatal("NewPasswordDigest(empty) returned a non-zero digest")
	}
}

func TestPasswordTypesDoNotExposeSecretsThroughFormattingOrJSON(t *testing.T) {
	const (
		plaintext = "$argon2id$Password1!"
		encoded   = "$argon2id$v=19$m=65536,t=3,p=4$secret-salt$secret-key"
	)
	password, err := NewPlainPassword(plaintext)
	if err != nil {
		t.Fatalf("NewPlainPassword() error = %v", err)
	}
	digest, err := NewPasswordDigest(encoded)
	if err != nil {
		t.Fatalf("NewPasswordDigest() error = %v", err)
	}

	for name, value := range map[string]any{
		"plaintext": password,
		"digest":    digest,
	} {
		t.Run(name, func(t *testing.T) {
			formatted := fmt.Sprintf("%s|%v|%+v|%#v", value, value, value, value)
			if strings.Contains(formatted, plaintext) || strings.Contains(formatted, encoded) {
				t.Fatal("password value leaked through formatting")
			}
			payload, marshalErr := json.Marshal(value)
			if marshalErr != nil {
				t.Fatalf("json.Marshal() error = %v", marshalErr)
			}
			if strings.Contains(string(payload), plaintext) || strings.Contains(string(payload), encoded) {
				t.Fatal("password value leaked through JSON")
			}
		})
	}
}

func TestPlainPasswordAndPasswordDigestHaveDistinctRepresentations(t *testing.T) {
	plainType := reflect.TypeOf(PlainPassword{})
	digestType := reflect.TypeOf(PasswordDigest{})
	if plainType == digestType || plainType.ConvertibleTo(digestType) || digestType.ConvertibleTo(plainType) {
		t.Fatal("plaintext passwords and password digests must not be interchangeable")
	}
}
