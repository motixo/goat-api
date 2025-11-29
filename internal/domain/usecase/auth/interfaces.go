package auth

import "context"

type UseCase interface {
	Login(ctx context.Context, input LoginInput) (LoginOutput, error)
	Signup(ctx context.Context, input RegisterInput) (RegisterOutput, error)
	Refresh(ctx context.Context, input RefreshInput) (RefreshOutput, error)
	Logout(ctx context.Context, token string) error
}
