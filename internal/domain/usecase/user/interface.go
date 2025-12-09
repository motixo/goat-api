package user

import (
	"context"

	"github.com/motixo/goat-api/internal/domain/pagination"
)

type UseCase interface {
	GetUser(ctx context.Context, userID string) (*UserResponse, error)
	DeleteUser(ctx context.Context, userID string) error
	UpdateUser(ctx context.Context, input UserUpdateInput) error
	GetUserslist(ctx context.Context, p pagination.Input) (*UserListResponse, error)
	ChangePassword(ctx context.Context, input UpdatePassInput) error
}
