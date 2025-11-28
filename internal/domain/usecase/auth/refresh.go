package auth

import (
	"context"
	"time"

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

func (a *AuthUseCase) Refresh(ctx context.Context, input RefreshInput) (RefreshOutput, error) {

	claims, err := valueobject.ParseAndValidate(input.RefreshToken, a.jwtSecret)
	if err != nil {
		return RefreshOutput{}, errors.ErrUnauthorized
	}

	if claims.TokenType != valueobject.TokenTypeRefresh {
		return RefreshOutput{}, errors.ErrUnauthorized
	}

	valid, err := a.jtiUC.IsJTIValid(ctx, claims.JTI)
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
		newJTI = a.ulidGen.New()
		newRefreshToken, newRefreshExp, err = valueobject.NewRefreshToken(claims.UserID, claims.Email, a.jwtSecret, newJTI)
		if err != nil {
			return RefreshOutput{}, err
		}
		if err := a.jtiUC.StoreJTI(ctx, jti.StoreInput{
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

	accessJTI := a.ulidGen.New()
	accessToken, accessExp, err := valueobject.NewAccessToken(claims.UserID, claims.Email, a.jwtSecret, accessJTI)
	if err != nil {
		return RefreshOutput{}, err
	}

	if err := a.jtiUC.StoreJTI(ctx, jti.StoreInput{
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
