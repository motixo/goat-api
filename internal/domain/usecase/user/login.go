package user

import (
	"context"
	"time"

	"github.com/mot0x0/gopi/internal/config"
	"github.com/mot0x0/gopi/internal/domain/errors"
	"github.com/mot0x0/gopi/internal/domain/valueobject"
)

type LoginInput struct {
	Email    string `json:"email" validate:"required,email"`
	Password string `json:"password" validate:"required"`
}

type LoginOutput struct {
	AccessToken           string       `json:"access_token"`
	AccessTokenExpiresAt  time.Time    `json:"access_token_expires_at"`
	RefreshToken          string       `json:"refresh_token"`
	RefreshTokenExpiresAt time.Time    `json:"refresh_token_expires_at"`
	User                  UserResponse `json:"user"`
}

func (u *UserUsecase) Login(ctx context.Context, input LoginInput) (LoginOutput, error) {
	user, err := u.userRepo.GetByEmail(ctx, input.Email)
	if err != nil {
		return LoginOutput{}, err
	}
	if user == nil {
		return LoginOutput{}, errors.ErrNotFound
	}

	p := valueobject.PasswordFromHash(user.Password)
	if !p.Check(input.Password) {
		return LoginOutput{}, errors.ErrUnauthorized
	}

	access, accessExp, err := valueobject.NewAccessToken(user.ID, user.Email, config.Get().JWTSecret)
	if err != nil {
		return LoginOutput{}, err
	}

	refresh, refreshExp, err := valueobject.NewRefreshToken(user.ID, user.Email, config.Get().JWTSecret)
	if err != nil {
		return LoginOutput{}, err
	}

	return LoginOutput{
		AccessToken:           access,
		AccessTokenExpiresAt:  accessExp,
		RefreshToken:          refresh,
		RefreshTokenExpiresAt: refreshExp,
		User: UserResponse{
			ID:        user.ID,
			Email:     user.Email,
			CreatedAt: user.CreatedAt,
		},
	}, nil
}
