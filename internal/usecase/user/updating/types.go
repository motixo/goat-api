package updating

import (
	"context"

	"github.com/motixo/goat-api/internal/domain/valueobject"
)

// UserUpdateCommand contains only the already-validated values that the
// generic User update persists. Role and status transitions deliberately use
// their dedicated workflows.
type UserUpdateCommand struct {
	UserID         string
	Email          string
	PasswordDigest valueobject.PasswordDigest
}

// UserUpdateWriter is owned by the generic User update application workflow.
type UserUpdateWriter interface {
	UpdateUser(ctx context.Context, command UserUpdateCommand) error
}
