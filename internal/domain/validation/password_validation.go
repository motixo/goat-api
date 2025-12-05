package validation

import (
	"unicode"

	"github.com/motixo/goat-api/internal/domain/errors"
)

func ValidatePasswordPolicy(plaintext string) error {
	if len(plaintext) < 8 {
		return errors.ErrPasswordTooShort
	}
	if len(plaintext) > 72 {
		return errors.ErrPasswordTooLong
	}

	var hasUpper, hasLower, hasDigit, hasSpecial bool
	for _, r := range plaintext {
		switch {
		case unicode.IsUpper(r):
			hasUpper = true
		case unicode.IsLower(r):
			hasLower = true
		case unicode.IsDigit(r):
			hasDigit = true
		case unicode.IsPunct(r) || unicode.IsSymbol(r):
			hasSpecial = true
		}
	}

	if !(hasUpper && hasLower && hasDigit && hasSpecial) {
		return errors.ErrPasswordPolicyViolation
	}
	return nil
}
