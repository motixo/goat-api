package listing

import (
	"context"
	"time"

	"github.com/motixo/goat-api/internal/domain/valueobject"
)

// UserListCriteria contains the effective authorization scope and filters that
// the User listing reader applies before counting and pagination.
type UserListCriteria struct {
	Statuses []valueobject.UserStatus
	Roles    []valueobject.UserRole
	Search   string
}

// UserListItem is the credential-free application projection returned by the
// User listing reader. It deliberately excludes password and credential state.
type UserListItem struct {
	ID        string
	Email     string
	Role      valueobject.UserRole
	Status    valueobject.UserStatus
	CreatedAt time.Time
}

type UserListResult struct {
	Items []UserListItem
	Total int64
}

// UserListReader is owned by the User listing application boundary. Its
// implementation must apply the supplied criteria before count and pagination.
type UserListReader interface {
	ListUsers(
		ctx context.Context,
		offset int,
		limit int,
		criteria UserListCriteria,
	) (UserListResult, error)
}
