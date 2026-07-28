package postgres

import (
	"context"
	"errors"
	"testing"

	"github.com/motixo/goat-api/internal/domain/valueobject"
)

func TestSeedAdminUserUsesCallerContext(t *testing.T) {
	cancellationCause := errors.New("database initialization canceled")
	ctx, cancel := context.WithCancelCause(context.Background())
	cancel(cancellationCause)
	hasher := seedPasswordHasher{
		hash: func(hashCtx context.Context, _ valueobject.PlainPassword) (valueobject.PasswordDigest, error) {
			if context.Cause(hashCtx) != cancellationCause {
				t.Fatalf("Hash() context cause = %v, want %v", context.Cause(hashCtx), cancellationCause)
			}
			return valueobject.PasswordDigest{}, cancellationCause
		},
	}

	err := SeedAdminUser(ctx, nil, hasher, "admin@goat.api", "Password1!")
	if !errors.Is(err, cancellationCause) {
		t.Fatalf("SeedAdminUser() error = %v, want caller cancellation cause", err)
	}
}

type seedPasswordHasher struct {
	hash func(context.Context, valueobject.PlainPassword) (valueobject.PasswordDigest, error)
}

func (h seedPasswordHasher) Hash(
	ctx context.Context,
	password valueobject.PlainPassword,
) (valueobject.PasswordDigest, error) {
	return h.hash(ctx, password)
}

func (seedPasswordHasher) Verify(
	context.Context,
	valueobject.PlainPassword,
	valueobject.PasswordDigest,
) (bool, error) {
	return false, nil
}
