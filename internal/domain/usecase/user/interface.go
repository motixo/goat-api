package user

import (
	"context"
)

type UseCase interface {
	GetUser(ctx context.Context, userID string) (*UserResponse, error)
	DeleteUser(ctx context.Context, userID string) error
	GetUserslist(ctx context.Context, actorID string, input GetListInput) ([]*UserResponse, int64, error)
	ChangeEmail(ctx context.Context, input UpdateEmailInput) error
	ChangePassword(ctx context.Context, input UpdatePassInput) error
	ChangeRole(ctx context.Context, input UpdateRoleInput) error
	ChangeStatus(ctx context.Context, input UpdateStatusInput) error
}
