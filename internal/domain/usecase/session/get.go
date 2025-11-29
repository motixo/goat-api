package session

import (
	"context"

	"github.com/mot0x0/gopi/internal/domain/entity"
)

func (s *SessionUseCase) GetSession(ctx context.Context, sessionID string) (*entity.Session, error) {
	return s.sessionRepo.Get(ctx, sessionID)
}
