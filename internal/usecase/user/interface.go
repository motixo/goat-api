package user

import (
	"context"
)

type UseCase interface {
	CreateUser(ctx context.Context, input CreateInput) (UserOutput, error)
	GetUser(ctx context.Context, userID string) (UserOutput, error)
	DeleteUser(ctx context.Context, userID string) error
	GetUserslist(ctx context.Context, input GetListInput) ([]UserOutput, int64, error)
	UpdateUser(ctx context.Context, input UpdateInput) error
	ChangeEmail(ctx context.Context, input UpdateEmailInput) error
	ChangePassword(ctx context.Context, input UpdatePassInput) error
	ChangeRole(ctx context.Context, input UpdateRoleInput) error
	ChangeStatus(ctx context.Context, input UpdateStatusInput) error
}

// PasswordChangeCleanupMetrics observes failures in non-authoritative cleanup
// that runs after a password and credential-version update has committed.
type PasswordChangeCleanupMetrics interface {
	RecordPasswordChangeCleanupFailure(stage string)
}
