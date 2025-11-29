package session

import (
	"context"
)

func (s *SessionUseCase) DeleteSessions(ctx context.Context, sessionIDs []string) error {
	return s.sessionRepo.Delete(ctx, sessionIDs)
}
