package auth

import (
	"context"

	"github.com/mot0x0/gopi/internal/domain/errors"
	"github.com/mot0x0/gopi/internal/domain/valueobject"
)

func (a *AuthUseCase) Logout(ctx context.Context, token string) error {

	accessClaims, err := valueobject.ParseAndValidate(token, a.jwtSecret)
	if err != nil {
		return errors.ErrUnauthorized
	}

	if err := a.sessionUC.DeleteSession(ctx, accessClaims.SessionID, accessClaims.JTI); err != nil {
		return err
	}
	return nil
}
