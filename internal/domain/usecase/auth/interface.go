package auth

import (
	"context"

	"github.com/motixo/goat-api/internal/domain/usecase/user"
)

type UseCase interface {
	Login(ctx context.Context, input LoginInput) (LoginOutput, error)
	Signup(ctx context.Context, input RegisterInput) (user.UserResponse, error)
	Refresh(ctx context.Context, input RefreshInput) (RefreshOutput, error)
	Logout(ctx context.Context, sessionID, userID string) error
}
