package authorization

import (
	"context"

	"github.com/motixo/goat-api/internal/domain/valueobject"
)

// SecurityStateReader is owned by the authorization application boundary. Its
// adapter must return current PostgreSQL-backed state rather than cached data.
type SecurityStateReader interface {
	GetSecurityState(ctx context.Context, userID string) (SecurityState, error)
}

// IdentityStateReader returns only current PostgreSQL-backed identity state for
// workflows that do not make a role or permission decision.
type IdentityStateReader interface {
	GetIdentityState(ctx context.Context, userID string) (IdentityState, error)
}

type UseCase interface {
	AuthorizeFreshIdentity(
		ctx context.Context,
		principal Principal,
	) (Principal, error)
	AuthorizeFresh(
		ctx context.Context,
		principal Principal,
		required valueobject.Permission,
	) (Principal, error)
}
