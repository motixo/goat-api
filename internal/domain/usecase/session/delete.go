package session

import (
	"context"
)

func (s *SessionUseCase) DeleteSession(ctx context.Context, sessionID, JTI string) error {
	return s.sessionRepo.Delete(ctx, sessionID, JTI)
}
