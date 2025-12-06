package errors

import "errors"

var (
	ErrPasswordTooShort        = errors.New("password must be at least 8 characters")
	ErrPasswordTooLong         = errors.New("password exceeds maximum length")
	ErrPasswordPolicyViolation = errors.New("password must contain uppercase, lowercase, number and special character")
	ErrPasswordHashingFailed   = errors.New("failed to hash password")
	ErrInvalidCredentials      = errors.New("invalid email or password")
	ErrInvalidPassword         = errors.New("password not match")
)
