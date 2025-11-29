package session

import (
	"context"

	"github.com/mot0x0/gopi/internal/domain/entity"
)

func (s *SessionUseCase) ListUserSessions(ctx context.Context, userID string) ([]*entity.Session, error) {
	return s.sessionRepo.ListByUser(ctx, userID)
}
