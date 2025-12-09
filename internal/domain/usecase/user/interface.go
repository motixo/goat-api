package user

import (
	"context"
)

type UseCase interface {
	GetUser(ctx context.Context, userID string) (*UserResponse, error)
	DeleteUser(ctx context.Context, userID string) error
	UpdateUser(ctx context.Context, input UserUpdateInput) error
	GetUserslist(ctx context.Context, offset, limit int) ([]*UserResponse, int64, error)
	ChangePassword(ctx context.Context, input UpdatePassInput) error
}
