package emailchange

import "context"

// UserEmailUpdateCommand contains only the values owned by the authenticated
// email-change workflow.
type UserEmailUpdateCommand struct {
	UserID string
	Email  string
}

// UserEmailUpdateWriter is owned by the email-change application workflow.
type UserEmailUpdateWriter interface {
	UpdateEmail(ctx context.Context, command UserEmailUpdateCommand) error
}
