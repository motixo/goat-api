package auth

import (
	"context"

	"github.com/motixo/goth-api/internal/domain/usecase/session"
)

func (us *AuthUseCase) Logout(ctx context.Context, sessionID, userID string) error {

	us.logger.Info("user logout requested", "userID", userID)

	input := session.DeleteSessionsInput{
		TargetSessions: []string{sessionID},
		UserID:         userID,
	}

	err := us.sessionUC.DeleteSessions(ctx, input)
	if err != nil {
		return err
	}
	us.logger.Info("user logged out", "userID", userID)
	return nil
}
