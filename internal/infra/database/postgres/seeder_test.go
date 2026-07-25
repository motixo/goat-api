package postgres

import (
	"context"
	"errors"
	"testing"

	"github.com/motixo/goat-api/internal/config"
	"github.com/motixo/goat-api/internal/domain/valueobject"
)

func TestSeedAdminUserUsesCallerContext(t *testing.T) {
	cancellationCause := errors.New("database initialization canceled")
	ctx, cancel := context.WithCancelCause(context.Background())
	cancel(cancellationCause)
	hasher := seedPasswordHasher{
		hash: func(hashCtx context.Context, _ string) (valueobject.Password, error) {
			if context.Cause(hashCtx) != cancellationCause {
				t.Fatalf("Hash() context cause = %v, want %v", context.Cause(hashCtx), cancellationCause)
			}
			return valueobject.Password{}, cancellationCause
		},
	}

	err := SeedAdminUser(ctx, nil, hasher, &config.Config{
		AdminEmail:    "admin@goat.api",
		AdminPassword: "password",
	})
	if !errors.Is(err, cancellationCause) {
		t.Fatalf("SeedAdminUser() error = %v, want caller cancellation cause", err)
	}
}

type seedPasswordHasher struct {
	hash func(context.Context, string) (valueobject.Password, error)
}

func (h seedPasswordHasher) Hash(
	ctx context.Context,
	plaintext string,
) (valueobject.Password, error) {
	return h.hash(ctx, plaintext)
}

func (seedPasswordHasher) Verify(context.Context, string, valueobject.Password) bool {
	return false
}

func (seedPasswordHasher) Validate(string) error {
	return nil
}
