package auth

import (
	"context"
)

func (a *AuthUseCase) Logout(ctx context.Context, sessionID string) error {

	if err := a.sessionUC.DeleteSessions(ctx, []string{sessionID}); err != nil {
		return err
	}
	return nil
}
