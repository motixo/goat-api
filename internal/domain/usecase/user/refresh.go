package user

import (
	"context"
	"time"

	"github.com/mot0x0/gopi/internal/config"
	"github.com/mot0x0/gopi/internal/domain/errors"
	"github.com/mot0x0/gopi/internal/domain/valueobject"
)

type RefreshInput struct {
	RefreshToken string `json:"refresh_token"`
}

type RefreshOutput struct {
	AccessToken           string    `json:"access_token"`
	AccessTokenExpiresAt  time.Time `json:"access_token_expires_at"`
	RefreshToken          string    `json:"refresh_token"`
	RefreshTokenExpiresAt time.Time `json:"refresh_token_expires_at"`
}

func (u *UserUsecase) Refresh(ctx context.Context, input RefreshInput) (RefreshOutput, error) {
	secret := config.Get().JWTSecret

	claims, err := valueobject.ParseAndValidate(input.RefreshToken, secret)
	if err != nil {
		return RefreshOutput{}, errors.ErrUnauthorized
	}

	if claims.TokenType != valueobject.TokenTypeRefresh {
		return RefreshOutput{}, errors.ErrUnauthorized
	}

	access, accessExp, err := valueobject.NewAccessToken(claims.UserID, claims.Email, secret)
	if err != nil {
		return RefreshOutput{}, err
	}

	return RefreshOutput{
		AccessToken:           access,
		AccessTokenExpiresAt:  accessExp,
		RefreshToken:          input.RefreshToken,
		RefreshTokenExpiresAt: claims.ExpiresAt.Time,
	}, nil
}
