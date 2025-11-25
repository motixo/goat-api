package user

import (
	"context"

	"github.com/mot0x0/gopi/internal/config"
	"github.com/mot0x0/gopi/internal/domain/errors"
	"github.com/mot0x0/gopi/internal/domain/valueobject"
)

type LogoutInput struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
}

func (u *UserUsecase) Logout(ctx context.Context, input LogoutInput) error {
	secret := config.Get().JWTSecret

	accessClaims, err := valueobject.ParseAndValidate(input.AccessToken, secret)
	if err != nil {
		return errors.ErrUnauthorized
	}

	refreshClaims, err := valueobject.ParseAndValidate(input.RefreshToken, secret)
	if err != nil {
		return errors.ErrUnauthorized
	}

	if err := u.jtiUC.RevokeJTI(ctx, accessClaims.JTI); err != nil {
		return err
	}

	if err := u.jtiUC.RevokeJTI(ctx, refreshClaims.JTI); err != nil {
		return err
	}

	return nil
}
