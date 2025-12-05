// internal/domain/valueobject/password.go
package valueobject

import (
	"github.com/motixo/goth-api/internal/domain/validation"
)

type Password struct {
	value string
}

func NewPassword(plaintext string) (Password, error) {
	if err := validation.ValidatePasswordPolicy(plaintext); err != nil {
		return Password{}, err
	}
	return Password{value: plaintext}, nil
}

func PasswordFromHash(hash string) Password {
	return Password{value: hash}
}

func (p Password) Value() string {
	return p.value
}

func (p Password) IsZero() bool {
	return p.value == ""
}

func (p Password) IsHashed() bool {
	// Check if it's a hashed password (starts with $)
	return len(p.value) > 0 && p.value[0] == '$'
}

func (p Password) Validate() error {
	if p.IsHashed() {
		return nil // Already hashed, assume valid
	}
	return validation.ValidatePasswordPolicy(p.value)
}
