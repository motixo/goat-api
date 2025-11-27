package session

import (
	"context"

	"github.com/mot0x0/gopi/internal/domain/entity"
)

func (s *SessionUsecase) Get(ctx context.Context, sessionID string) (*entity.Session, error) {
	return s.sessionRepo.GetSession(ctx, sessionID)
}
