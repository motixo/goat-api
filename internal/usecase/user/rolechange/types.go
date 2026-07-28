package rolechange

import (
	"context"
	"errors"

	"github.com/motixo/goat-api/internal/domain/valueobject"
)

// ErrConcurrentRoleChange reports that the authoritative target role changed
// after the application validated its role hierarchy.
var ErrConcurrentRoleChange = errors.New("user role changed concurrently")

// UserRoleUpdateOutcome classifies the authoritative result of the role
// compare-and-set without collapsing distinct zero-row outcomes into a bool.
type UserRoleUpdateOutcome uint8

const (
	UserRoleUpdateUnknown UserRoleUpdateOutcome = iota
	UserRoleUpdateApplied
	UserRoleUpdateAlreadyApplied
	UserRoleUpdateConflict
	UserRoleUpdateNotFound
)

// UserRoleUpdateResult reports whether this request committed the role change
// and, when present, the authoritative role observed by the writer.
type UserRoleUpdateResult struct {
	Outcome     UserRoleUpdateOutcome
	CurrentRole valueobject.UserRole
}

// UserRoleUpdateCommand contains only the validated values owned by the
// dedicated role-change workflow.
type UserRoleUpdateCommand struct {
	UserID        string
	ExpectedRole  valueobject.UserRole
	RequestedRole valueobject.UserRole
}

// UserRoleUpdateWriter is owned by the role-change application workflow.
type UserRoleUpdateWriter interface {
	UpdateRole(
		ctx context.Context,
		command UserRoleUpdateCommand,
	) (UserRoleUpdateResult, error)
}
