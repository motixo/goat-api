package auth

import (
	"context"

	"github.com/mot0x0/goth-api/internal/domain/usecase/session"
)

func (a *AuthUseCase) Logout(ctx context.Context, sessionID string) error {

	input := session.DeleteSessionsInput{
		TargetSessions: []string{sessionID},
	}

	return a.sessionUC.DeleteSessions(ctx, input)
}
