package service

import (
	"context"

	domainErrors "github.com/motixo/goat-api/internal/domain/errors"
	"github.com/motixo/goat-api/internal/domain/valueobject"
)

// ErrInvalidStoredPasswordHash identifies a persisted password credential that
// cannot be verified safely. Callers must not treat it as an ordinary password
// mismatch, and delivery must not expose its cause or representation.
var ErrInvalidStoredPasswordHash = domainErrors.ErrInvalidStoredPasswordHash

type PasswordHasher interface {
	// Hash and Verify may return a caller-context error before password work is
	// admitted. Once the synchronous password derivation starts, cancellation
	// does not interrupt it and the completed result is returned normally.
	Hash(ctx context.Context, password valueobject.PlainPassword) (valueobject.PasswordDigest, error)
	// Verify returns (false, nil) only for a valid encoded hash and an incorrect
	// plaintext. Malformed or unsupported persisted hashes wrap
	// ErrInvalidStoredPasswordHash before any expensive derivation is admitted.
	Verify(
		ctx context.Context,
		password valueobject.PlainPassword,
		digest valueobject.PasswordDigest,
	) (bool, error)
}
