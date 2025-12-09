// internal/domain/valueobject/password.go
package valueobject

import (
	"database/sql/driver"
	"fmt"

	"github.com/motixo/goat-api/internal/domain/validation"
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

func (p Password) Value() (driver.Value, error) {
	return p.value, nil
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

func (p *Password) Scan(value any) error {
	if value == nil {
		p.value = ""
		return nil
	}
	str, ok := value.(string)
	if !ok {
		return fmt.Errorf("cannot scan %T into Password", value)
	}
	p.value = str
	return nil
}
