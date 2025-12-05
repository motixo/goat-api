package service

import (
	"context"

	"github.com/motixo/goat-api/internal/domain/valueobject"
)

type PasswordHasher interface {
	Hash(ctx context.Context, plaintext string) (valueobject.Password, error)
	Verify(ctx context.Context, plaintext string, hashed valueobject.Password) bool
	Validate(plaintext string) error
}
