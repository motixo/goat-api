package statuschange

import (
	"context"

	"github.com/motixo/goat-api/internal/domain/valueobject"
)

// UserStatusSnapshot contains the authoritative target state required by User
// command preconditions that must inspect role or status without loading
// credential data.
type UserStatusSnapshot struct {
	ID     string
	Role   valueobject.UserRole
	Status valueobject.UserStatus
}

// UserStatusSnapshotReader is the shared application-owned boundary for those
// credential-free command preconditions. Status mutation remains the separate
// compare-and-set operation owned by the status-transition workflow.
type UserStatusSnapshotReader interface {
	FindStatusSnapshotByID(
		ctx context.Context,
		id string,
	) (UserStatusSnapshot, error)
}
