// internal/domain/valueobject/password.go
package valueobject

import (
	domainErrors "github.com/motixo/goat-api/internal/domain/errors"
	"github.com/motixo/goat-api/internal/domain/validation"
)

const redactedPassword = "<redacted>"

// PlainPassword is a validated password supplied for hashing or verification.
// Its plaintext representation is deliberately available only through Bytes;
// formatting and JSON reflection cannot expose the unexported value.
type PlainPassword struct {
	plaintext string
}

func NewPlainPassword(plaintext string) (PlainPassword, error) {
	if err := validation.ValidatePasswordPolicy(plaintext); err != nil {
		return PlainPassword{}, err
	}
	return PlainPassword{plaintext: plaintext}, nil
}

func (p PlainPassword) Bytes() []byte {
	return []byte(p.plaintext)
}

func (p PlainPassword) IsZero() bool {
	return p.plaintext == ""
}

func (p PlainPassword) Validate() error {
	return validation.ValidatePasswordPolicy(p.plaintext)
}

func (PlainPassword) String() string {
	return redactedPassword
}

func (p PlainPassword) GoString() string {
	return p.String()
}

// PasswordDigest is an opaque encoded credential produced by a password
// hasher or rehydrated by a persistence adapter. Full algorithm, version, and
// resource validation belongs to the password infrastructure adapter.
type PasswordDigest struct {
	encoded string
}

func NewPasswordDigest(encoded string) (PasswordDigest, error) {
	if encoded == "" {
		return PasswordDigest{}, domainErrors.ErrInvalidStoredPasswordHash
	}
	return PasswordDigest{encoded: encoded}, nil
}

func (d PasswordDigest) Encoded() string {
	return d.encoded
}

func (d PasswordDigest) IsZero() bool {
	return d.encoded == ""
}

func (PasswordDigest) String() string {
	return redactedPassword
}

func (d PasswordDigest) GoString() string {
	return d.String()
}
