package auth

import (
	"context"

	"github.com/mot0x0/gopi/internal/domain/errors"
	"github.com/mot0x0/gopi/internal/domain/valueobject"
)

type LogoutInput struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
}

func (a *AuthUseCase) Logout(ctx context.Context, input LogoutInput) error {

	accessClaims, err := valueobject.ParseAndValidate(input.AccessToken, a.jwtSecret)
	if err != nil {
		return errors.ErrUnauthorized
	}

	refreshClaims, err := valueobject.ParseAndValidate(input.RefreshToken, a.jwtSecret)
	if err != nil {
		return errors.ErrUnauthorized
	}

	if err := a.jtiUC.RevokeJTI(ctx, accessClaims.JTI); err != nil {
		return err
	}

	if err := a.jtiUC.RevokeJTI(ctx, refreshClaims.JTI); err != nil {
		return err
	}

	return nil
}
