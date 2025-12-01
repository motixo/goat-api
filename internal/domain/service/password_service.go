package service

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"fmt"
	"strings"
	"unicode"

	"github.com/mot0x0/goth-api/internal/config"
	"github.com/mot0x0/goth-api/internal/domain/errors"
	"github.com/mot0x0/goth-api/internal/domain/valueobject"
	"golang.org/x/crypto/argon2"
)

const (
	argonTime    = 3
	argonMemory  = 64 * 1024
	argonThreads = 4
	argonKeyLen  = 32
	saltLen      = 32
)

type PasswordHasher interface {
	Hash(ctx context.Context, plaintext string) (valueobject.Password, error)
	Verify(ctx context.Context, plaintext string, hashed valueobject.Password) bool
}

// PasswordService handles password hashing and verification
type PasswordService struct {
	pepper string
}

// NewPasswordService creates a service with injected pepper
func NewPasswordService(cfg *config.Config) PasswordHasher {
	return &PasswordService{
		pepper: cfg.PasswordPepper,
	}
}

// Hash creates a Password from plaintext
func (s *PasswordService) Hash(ctx context.Context, plain string) (valueobject.Password, error) {
	if err := s.validatePolicy(plain); err != nil {
		return valueobject.Password{}, err
	}

	salt := make([]byte, saltLen)
	if _, err := rand.Read(salt); err != nil {
		return valueobject.Password{}, errors.ErrPasswordHashingFailed
	}

	// Apply pepper before hashing
	input := append([]byte(plain), []byte(s.pepper)...)
	hash := argon2.IDKey(input, salt, argonTime, argonMemory, argonThreads, argonKeyLen)

	encoded := fmt.Sprintf("$argon2id$v=19$m=%d,t=%d,p=%d$%s$%s",
		argonMemory, argonTime, argonThreads,
		base64.RawStdEncoding.EncodeToString(salt),
		base64.RawStdEncoding.EncodeToString(hash))

	return valueobject.PasswordFromHash(encoded), nil
}

// Verify checks if plaintext matches hashed password
func (s *PasswordService) Verify(ctx context.Context, plain string, hashed valueobject.Password) bool {
	parts := strings.Split(hashed.Value(), "$")
	if len(parts) != 6 {
		return false
	}

	var mem uint32
	var time, threads uint8
	_, err := fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &mem, &time, &threads)
	if err != nil {
		return false
	}

	salt, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil {
		return false
	}

	expected, err := base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil {
		return false
	}

	// Apply same pepper for verification
	input := append([]byte(plain), []byte(s.pepper)...)
	hash := argon2.IDKey(input, salt, uint32(time), mem, uint8(threads), uint32(len(expected)))

	return subtle.ConstantTimeCompare(hash, expected) == 1
}

func (s *PasswordService) validatePolicy(p string) error {
	if len(p) < 8 {
		return errors.ErrPasswordTooShort
	}
	if len(p) > 72 {
		return errors.ErrPasswordTooLong
	}

	var hasUpper, hasLower, hasDigit, hasSpecial bool
	for _, r := range p {
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
