package service

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"fmt"
	"strings"

	"github.com/motixo/goat-api/internal/config"
	"github.com/motixo/goat-api/internal/domain/errors"
	"github.com/motixo/goat-api/internal/domain/service"
	"github.com/motixo/goat-api/internal/domain/validation"
	"github.com/motixo/goat-api/internal/domain/valueobject"
	"golang.org/x/crypto/argon2"
)

const (
	argonTime    = 3
	argonMemory  = 64 * 1024
	argonThreads = 4
	argonKeyLen  = 32
	saltLen      = 32
)

type PasswordService struct {
	pepper string
}

func NewPasswordService(cfg *config.Config) service.PasswordHasher {
	return &PasswordService{
		pepper: cfg.PasswordPepper,
	}
}

func (s *PasswordService) Hash(ctx context.Context, plaintext string) (valueobject.Password, error) {
	// Validate the plaintext password
	if err := s.Validate(plaintext); err != nil {
		return valueobject.Password{}, err
	}

	salt := make([]byte, saltLen)
	if _, err := rand.Read(salt); err != nil {
		return valueobject.Password{}, errors.ErrPasswordHashingFailed
	}

	// Apply pepper before hashing
	input := append([]byte(plaintext), []byte(s.pepper)...)
	hash := argon2.IDKey(input, salt, argonTime, argonMemory, argonThreads, argonKeyLen)

	encoded := fmt.Sprintf("$argon2id$v=19$m=%d,t=%d,p=%d$%s$%s",
		argonMemory, argonTime, argonThreads,
		base64.RawStdEncoding.EncodeToString(salt),
		base64.RawStdEncoding.EncodeToString(hash))

	return valueobject.PasswordFromHash(encoded), nil
}

func (s *PasswordService) Verify(ctx context.Context, plaintext string, hashed valueobject.Password) bool {

	val, err := hashed.Value()
	if err != nil {
		return false
	}
	parts := strings.Split(val.(string), "$")
	if len(parts) != 6 {
		return false
	}

	var mem uint32
	var time, threads uint8
	_, serr := fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &mem, &time, &threads)
	if serr != nil {
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
	input := append([]byte(plaintext), []byte(s.pepper)...)
	hash := argon2.IDKey(input, salt, uint32(time), mem, uint8(threads), uint32(len(expected)))

	return subtle.ConstantTimeCompare(hash, expected) == 1
}

func (s *PasswordService) Validate(plaintext string) error {
	return validation.ValidatePasswordPolicy(plaintext)
}
