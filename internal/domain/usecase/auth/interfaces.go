package auth

import "context"

type AuthUseCase interface {
	Login(ctx context.Context, input LoginInput) (LoginOutput, error)
	Refresh(ctx context.Context, input RefreshInput) (RefreshOutput, error)
	Logout(ctx context.Context, input LogoutInput) error
}
