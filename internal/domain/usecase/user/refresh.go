package user

import (
	"context"
	"time"

	"github.com/mot0x0/gopi/internal/config"
	"github.com/mot0x0/gopi/internal/domain/errors"
	"github.com/mot0x0/gopi/internal/domain/usecase/jti"
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

	valid, err := u.jtiUC.IsJTIValid(ctx, claims.JTI)
	if err != nil {
		return RefreshOutput{}, err
	}
	if !valid {
		return RefreshOutput{}, errors.ErrUnauthorized
	}

	now := time.Now().UTC()
	refreshRemaining := time.Until(claims.ExpiresAt.Time)

	var newRefreshToken string
	var newRefreshExp time.Time
	var newJTI string

	if refreshRemaining < 24*time.Hour {
		newRefreshToken, newJTI, newRefreshExp, err = valueobject.NewRefreshToken(claims.UserID, claims.Email, secret)
		if err != nil {
			return RefreshOutput{}, err
		}
		if err := u.jtiUC.StoreJTI(ctx, jti.StoreInput{
			UserID: claims.UserID,
			JTI:    newJTI,
			Exp:    newRefreshExp.Sub(now),
		}); err != nil {
			return RefreshOutput{}, err
		}
	} else {
		newRefreshToken = input.RefreshToken
		newRefreshExp = claims.ExpiresAt.Time
	}

	accessToken, accessJTI, accessExp, err := valueobject.NewAccessToken(claims.UserID, claims.Email, secret)
	if err != nil {
		return RefreshOutput{}, err
	}

	if err := u.jtiUC.StoreJTI(ctx, jti.StoreInput{
		UserID: claims.UserID,
		JTI:    accessJTI,
		Exp:    accessExp.Sub(now),
	}); err != nil {
		return RefreshOutput{}, err
	}

	return RefreshOutput{
		AccessToken:           accessToken,
		AccessTokenExpiresAt:  accessExp,
		RefreshToken:          newRefreshToken,
		RefreshTokenExpiresAt: newRefreshExp,
	}, nil
}
